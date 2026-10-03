package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

type analysisHandler struct {
	svc port.AnalysisService
}

// maxRankLimit caps a ranking page.
const maxRankLimit = 200

// analyze handles GET /api/v1/assets/{market}/{symbol}/analysis?profile=moderate.
func (h analysisHandler) analyze(w http.ResponseWriter, r *http.Request) {
	key, err := parseAssetKey(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	p, err := domain.ParseRiskProfile(r.URL.Query().Get("profile"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	a, err := h.svc.Analyze(r.Context(), key, p)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAnalysisDTO(a))
}

// rank handles GET /api/v1/rankings?market=B3&class=stock&profile=moderate&limit=50.
func (h analysisHandler) rank(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	af, err := parseAssetFilter(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	p, err := domain.ParseRiskProfile(q.Get("profile"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	f := port.RankFilter{Market: af.Market, Class: af.Class, Profile: p, Limit: 50}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxRankLimit {
			writeDomainError(w, fmt.Errorf("%w: limit must be 1 to %d", domain.ErrInvalid, maxRankLimit))
			return
		}
		f.Limit = n
	}
	rows, err := h.svc.Rank(r.Context(), f)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := rankingDTO{Profile: string(p), Items: make([]rankedDTO, len(rows)), Disclaimer: domain.Disclaimer}
	for i, row := range rows {
		out.Items[i] = toRankedDTO(row)
	}
	writeJSON(w, http.StatusOK, out)
}

// outlook handles GET /api/v1/outlook?profile=conservative.
func (h analysisHandler) outlook(w http.ResponseWriter, r *http.Request) {
	p, err := domain.ParseRiskProfile(r.URL.Query().Get("profile"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	o, err := h.svc.Outlook(r.Context(), p)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := outlookDTO{Profile: string(o.Profile), AsOf: dateString(o.AsOf), Bands: make([]bandDTO, len(o.Bands)),
		Signals: toSignalDTOs(o.Signals), Notes: nonNil(o.Notes), Disclaimer: domain.Disclaimer}
	for i, b := range o.Bands {
		out.Bands[i] = bandDTO{Class: b.Class, Min: b.Min, Max: b.Max, Lean: b.Lean}
	}
	writeJSON(w, http.StatusOK, out)
}

func parseAssetKey(r *http.Request) (domain.AssetKey, error) {
	market, err := domain.ParseMarket(r.PathValue("market"))
	if err != nil {
		return domain.AssetKey{}, err
	}
	symbol, err := domain.NewSymbol(r.PathValue("symbol"))
	if err != nil {
		return domain.AssetKey{}, err
	}
	return domain.AssetKey{Market: market, Symbol: symbol}, nil
}
