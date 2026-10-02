package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

type catalogHandler struct {
	svc port.CatalogService
}

// list handles GET /api/v1/assets?market=B3&class=stock&indexMember=true.
func (h catalogHandler) list(w http.ResponseWriter, r *http.Request) {
	f, err := parseAssetFilter(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	assets, err := h.svc.ListAssets(r.Context(), f)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]assetDTO, len(assets))
	for i, a := range assets {
		items[i] = toAssetDTO(a)
	}
	writeJSON(w, http.StatusOK, listDTO[assetDTO]{Items: items})
}

// get handles GET /api/v1/assets/{market}/{symbol}.
func (h catalogHandler) get(w http.ResponseWriter, r *http.Request) {
	market, err := domain.ParseMarket(r.PathValue("market"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	symbol, err := domain.NewSymbol(r.PathValue("symbol"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	a, err := h.svc.GetAsset(r.Context(), domain.AssetKey{Market: market, Symbol: symbol})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAssetDTO(a))
}

func parseAssetFilter(r *http.Request) (port.AssetFilter, error) {
	q := r.URL.Query()
	var f port.AssetFilter
	if v := q.Get("market"); v != "" {
		m, err := domain.ParseMarket(v)
		if err != nil {
			return f, err
		}
		f.Market = m
	}
	if v := q.Get("class"); v != "" {
		c, err := domain.ParseAssetClass(v)
		if err != nil {
			return f, err
		}
		f.Class = c
	}
	if v := q.Get("indexMember"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return f, fmt.Errorf("%w: indexMember must be true or false", domain.ErrInvalid)
		}
		f.IndexMember = &b
	}
	return f, nil
}
