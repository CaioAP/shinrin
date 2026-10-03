package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

type watchlistHandler struct {
	svc port.WatchlistService
}

type nameBody struct {
	Name string `json:"name"`
}

// list handles GET /api/v1/watchlists.
func (h watchlistHandler) list(w http.ResponseWriter, r *http.Request, u domain.User) {
	ls, err := h.svc.List(r.Context(), u.ID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]watchlistDTO, len(ls))
	for i, l := range ls {
		out[i] = toWatchlistDTO(l)
	}
	writeJSON(w, http.StatusOK, listDTO[watchlistDTO]{Items: out})
}

// create handles POST /api/v1/watchlists.
func (h watchlistHandler) create(w http.ResponseWriter, r *http.Request, u domain.User) {
	var b nameBody
	if err := decodeJSON(w, r, &b); err != nil {
		writeDomainError(w, err)
		return
	}
	l, err := h.svc.Create(r.Context(), u.ID, b.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toWatchlistDTO(l))
}

// get handles GET /api/v1/watchlists/{id}?profile=moderate: the list with
// each asset's scores. The profile defaults to the user's own.
func (h watchlistHandler) get(w http.ResponseWriter, r *http.Request, u domain.User) {
	id, err := watchlistID(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	p := u.Profile
	if v := r.URL.Query().Get("profile"); v != "" || p == "" {
		if p, err = domain.ParseRiskProfile(v); err != nil {
			writeDomainError(w, err)
			return
		}
	}
	l, entries, err := h.svc.Entries(r.Context(), u.ID, id, p)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := watchlistEntriesDTO{ID: int64(l.ID), Name: l.Name, Profile: string(p), Items: make([]watchlistEntryDTO, len(entries)), Disclaimer: domain.Disclaimer}
	for i, e := range entries {
		out.Items[i] = watchlistEntryDTO{Asset: toAssetDTO(e.Asset)}
		if e.Ranked != nil {
			rd := toRankedDTO(*e.Ranked)
			out.Items[i].Scores = &rd
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// rename handles PATCH /api/v1/watchlists/{id}.
func (h watchlistHandler) rename(w http.ResponseWriter, r *http.Request, u domain.User) {
	id, err := watchlistID(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	var b nameBody
	if err := decodeJSON(w, r, &b); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := h.svc.Rename(r.Context(), u.ID, id, b.Name); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// remove handles DELETE /api/v1/watchlists/{id}.
func (h watchlistHandler) remove(w http.ResponseWriter, r *http.Request, u domain.User) {
	id, err := watchlistID(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := h.svc.Delete(r.Context(), u.ID, id); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addItem handles PUT /api/v1/watchlists/{id}/items/{market}/{symbol}.
func (h watchlistHandler) addItem(w http.ResponseWriter, r *http.Request, u domain.User) {
	h.item(w, r, u, h.svc.AddAsset)
}

// removeItem handles DELETE /api/v1/watchlists/{id}/items/{market}/{symbol}.
func (h watchlistHandler) removeItem(w http.ResponseWriter, r *http.Request, u domain.User) {
	h.item(w, r, u, h.svc.RemoveAsset)
}

type itemOp func(ctx context.Context, user domain.UserID, id domain.WatchlistID, asset domain.AssetKey) error

func (h watchlistHandler) item(w http.ResponseWriter, r *http.Request, u domain.User, op itemOp) {
	id, err := watchlistID(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	key, err := parseAssetKey(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := op(r.Context(), u.ID, id, key); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func watchlistID(r *http.Request) (domain.WatchlistID, error) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%w: watchlist id must be a positive integer", domain.ErrInvalid)
	}
	return domain.WatchlistID(n), nil
}
