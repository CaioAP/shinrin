package httpapi

import "github.com/CaioAP/shinrin/backend/internal/domain"

// DTOs decouple the JSON contract from domain structs, so renaming a domain
// field never silently breaks the web app. Keep them in sync with
// web/shared/types/api.ts.

type metaDTO struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Disclaimer string `json:"disclaimer"`
}

type healthDTO struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

type assetDTO struct {
	Market      string `json:"market"`
	Symbol      string `json:"symbol"`
	Class       string `json:"class"`
	Name        string `json:"name"`
	Sector      string `json:"sector,omitempty"`
	Currency    string `json:"currency"`
	IndexMember bool   `json:"indexMember"`
	Active      bool   `json:"active"`
}

type listDTO[T any] struct {
	Items []T `json:"items"`
}

func toAssetDTO(a domain.Asset) assetDTO {
	return assetDTO{
		Market:      string(a.Key.Market),
		Symbol:      string(a.Key.Symbol),
		Class:       string(a.Class),
		Name:        a.Name,
		Sector:      a.Sector,
		Currency:    a.Currency(),
		IndexMember: a.IndexMember,
		Active:      a.Active,
	}
}
