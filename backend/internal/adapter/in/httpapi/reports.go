package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// aiHandler serves a user's own LLM key settings and AI reports.
type aiHandler struct {
	creds   port.CredentialService
	reports port.UserReportService
}

type llmBody struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	BaseURL    string `json:"baseUrl"`
	APIKey     string `json:"apiKey"`
	MonthlyCap int    `json:"monthlyCap"`
}

// settings handles GET /api/v1/me/llm.
func (h aiHandler) settings(w http.ResponseWriter, r *http.Request, u domain.User) {
	out, err := h.settingsDTO(r, u)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h aiHandler) settingsDTO(r *http.Request, u domain.User) (llmSettingsDTO, error) {
	out := llmSettingsDTO{Available: true}
	set, err := h.creds.Settings(r.Context(), u.ID)
	switch {
	case errors.Is(err, domain.ErrUnavailable):
		out.Available = false
	case errors.Is(err, domain.ErrNotFound):
	case err != nil:
		return llmSettingsDTO{}, err
	default:
		out.Configured, out.Provider, out.Model, out.BaseURL, out.KeyHint, out.MonthlyCap =
			true, set.Provider, set.Model, set.BaseURL, set.KeyHint, set.MonthlyCap
		out.UpdatedAt = set.UpdatedAt.UTC().Format(timeFormat)
	}
	usage, err := h.reports.Usage(r.Context(), u.ID)
	if err != nil {
		return llmSettingsDTO{}, err
	}
	out.Usage = llmUsageDTO{Used: usage.Used, Cap: usage.Cap, Since: dateString(usage.Since)}
	return out, nil
}

// save handles PUT /api/v1/me/llm. An empty apiKey keeps the saved key.
func (h aiHandler) save(w http.ResponseWriter, r *http.Request, u domain.User) {
	var b llmBody
	if err := decodeJSON(w, r, &b); err != nil {
		writeDomainError(w, err)
		return
	}
	in := port.CredentialInput{Provider: b.Provider, Model: b.Model, BaseURL: b.BaseURL, APIKey: b.APIKey, MonthlyCap: b.MonthlyCap}
	if _, err := h.creds.Save(r.Context(), u.ID, in); err != nil {
		writeDomainError(w, err)
		return
	}
	h.settings(w, r, u)
}

// test handles POST /api/v1/me/llm/test.
func (h aiHandler) test(w http.ResponseWriter, r *http.Request, u domain.User) {
	if err := h.creds.Test(r.Context(), u.ID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// remove handles DELETE /api/v1/me/llm.
func (h aiHandler) remove(w http.ResponseWriter, r *http.Request, u domain.User) {
	if err := h.creds.Delete(r.Context(), u.ID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// list handles GET /api/v1/assets/{market}/{symbol}/reports: the user's
// own reports on the asset, newest first.
func (h aiHandler) list(w http.ResponseWriter, r *http.Request, u domain.User) {
	k, err := parseAssetKey(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	limit, err := intParam(r, "limit", 10, 20)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	rs, err := h.reports.List(r.Context(), u.ID, k, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]reportDTO, len(rs))
	for i, x := range rs {
		out[i] = toReportDTO(x)
	}
	writeJSON(w, http.StatusOK, listDTO[reportDTO]{Items: out})
}

type generateBody struct {
	Profile string `json:"profile"`
	Lang    string `json:"lang"`
}

// generate handles POST /api/v1/assets/{market}/{symbol}/reports. The
// profile defaults to the user's own, then moderate.
func (h aiHandler) generate(w http.ResponseWriter, r *http.Request, u domain.User) {
	k, err := parseAssetKey(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	var b generateBody
	if err := decodeJSON(w, r, &b); err != nil {
		writeDomainError(w, err)
		return
	}
	if b.Profile == "" {
		b.Profile = string(u.Profile)
	}
	p, err := domain.ParseRiskProfile(b.Profile)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	rep, err := h.reports.Generate(r.Context(), u.ID, k, p, b.Lang)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toReportDTO(rep))
}

// get handles GET /api/v1/reports/{id}.
func (h aiHandler) get(w http.ResponseWriter, r *http.Request, u domain.User) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeDomainError(w, fmt.Errorf("%w: bad report id", domain.ErrInvalid))
		return
	}
	rep, err := h.reports.Get(r.Context(), u.ID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReportDTO(rep))
}
