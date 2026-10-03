package httpapi

import (
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

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

// Analysis DTOs. Every analysis response carries the disclaimer, so no
// client can show a score without it.

type scoreInputDTO struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
	Points float64 `json:"points"`
	Weight float64 `json:"weight"`
	Peers  int     `json:"peers,omitempty"`
}

type factorDTO struct {
	Factor    string          `json:"factor"`
	Value     float64         `json:"value"`
	PeerGroup string          `json:"peerGroup"`
	Inputs    []scoreInputDTO `json:"inputs"`
}

type signalDTO struct {
	Code    string             `json:"code"`
	Message string             `json:"message"`
	Weight  float64            `json:"weight"`
	Data    map[string]float64 `json:"data,omitempty"`
}

type fairValueDTO struct {
	Method      string             `json:"method"`
	Low         float64            `json:"low"`
	High        float64            `json:"high"`
	Assumptions map[string]float64 `json:"assumptions"`
}

type viewDTO struct {
	View    string      `json:"view"`
	Signals []signalDTO `json:"signals"`
}

type analysisDTO struct {
	Asset      assetDTO           `json:"asset"`
	AsOf       string             `json:"asOf"`
	Profile    string             `json:"profile"`
	Price      float64            `json:"price"`
	Composite  float64            `json:"composite"`
	Coverage   float64            `json:"coverage"`
	Factors    []factorDTO        `json:"factors"`
	Indicators map[string]float64 `json:"indicators"`
	FairValues []fairValueDTO     `json:"fairValues"`
	Valuation  viewDTO            `json:"valuation"`
	Timing     viewDTO            `json:"timing"`
	Notes      []string           `json:"notes"`
	Disclaimer string             `json:"disclaimer"`
}

type rankedDTO struct {
	Asset     assetDTO           `json:"asset"`
	AsOf      string             `json:"asOf"`
	Composite float64            `json:"composite"`
	Coverage  float64            `json:"coverage"`
	Factors   map[string]float64 `json:"factors"`
}

type rankingDTO struct {
	Profile    string      `json:"profile"`
	Items      []rankedDTO `json:"items"`
	Disclaimer string      `json:"disclaimer"`
}

type bandDTO struct {
	Class string  `json:"class"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Lean  string  `json:"lean"`
}

type outlookDTO struct {
	Profile    string      `json:"profile"`
	AsOf       string      `json:"asOf,omitempty"`
	Bands      []bandDTO   `json:"bands"`
	Signals    []signalDTO `json:"signals"`
	Notes      []string    `json:"notes"`
	Disclaimer string      `json:"disclaimer"`
}

func toAnalysisDTO(a domain.Analysis) analysisDTO {
	out := analysisDTO{
		Asset: toAssetDTO(a.Asset), AsOf: dateString(a.AsOf), Profile: string(a.Profile), Price: a.Price,
		Composite: a.Composite, Coverage: a.Coverage, Factors: make([]factorDTO, len(a.Scorecard.Factors)),
		Indicators: a.Indicators.Values, FairValues: make([]fairValueDTO, len(a.FairValues)),
		Valuation: viewDTO{View: string(a.Valuation), Signals: toSignalDTOs(a.ValuationSignals)},
		Timing:    viewDTO{View: string(a.Timing.View), Signals: toSignalDTOs(a.Timing.Signals)},
		Notes:     nonNil(a.Notes), Disclaimer: domain.Disclaimer,
	}
	if out.Indicators == nil {
		out.Indicators = map[string]float64{}
	}
	for i, f := range a.Scorecard.Factors {
		in := make([]scoreInputDTO, len(f.Inputs))
		for j, x := range f.Inputs {
			in[j] = scoreInputDTO{x.Metric, x.Value, x.Points, x.Weight, x.Peers}
		}
		out.Factors[i] = factorDTO{Factor: string(f.Factor), Value: f.Value, PeerGroup: f.PeerGroup, Inputs: in}
	}
	for i, fv := range a.FairValues {
		out.FairValues[i] = fairValueDTO{fv.Method, fv.Low, fv.High, fv.Assumptions}
	}
	return out
}

func toSignalDTOs(s []domain.Signal) []signalDTO {
	out := make([]signalDTO, len(s))
	for i, x := range s {
		out[i] = signalDTO{x.Code, x.Message, x.Weight, x.Data}
	}
	return out
}

func dateString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
