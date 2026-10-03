// Package report holds the pure rules of AI reports (docs/design.md, section
// 9): what goes into the input snapshot the LLM sees, and how its output is
// checked against that snapshot so the model cannot invent figures.
package report

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/scoring"
)

// BandClassFor maps an asset class to the allocation band it draws from.
func BandClassFor(c domain.AssetClass) string {
	switch c {
	case domain.ClassFII, domain.ClassREIT:
		return scoring.ClassRealEstate
	case domain.ClassGovBond:
		return scoring.ClassFixedIncome
	}
	return scoring.ClassStocks
}

// NewSnapshot builds a report's input from the engine's analysis, the
// profile's outlook and recent headlines (newest first, at most
// maxHeadlines). Numbers are rounded to what a reader would quote.
func NewSnapshot(a domain.Analysis, o domain.Outlook, news []domain.NewsItem, maxHeadlines int) domain.ReportSnapshot {
	s := domain.ReportSnapshot{Facts: map[string]float64{}, Labels: map[string]string{}}
	fact := func(k string, v float64) {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			s.Facts[k] = round(v, 4)
		}
	}

	s.Labels["symbol"] = string(a.Asset.Key.Symbol)
	s.Labels["market"] = string(a.Asset.Key.Market)
	s.Labels["name"] = a.Asset.Name
	s.Labels["class"] = string(a.Asset.Class)
	s.Labels["currency"] = a.Asset.Currency()
	if a.Asset.Sector != "" {
		s.Labels["sector"] = a.Asset.Sector
	}
	s.Labels["as_of"] = a.AsOf.Format("2006-01-02")
	s.Labels["risk_profile"] = string(a.Profile)
	s.Labels["valuation_view"] = string(a.Valuation)
	s.Labels["timing_view"] = string(a.Timing.View)

	fact("price", a.Price)
	fact("composite_score", a.Composite)
	fact("score_coverage", a.Coverage)
	for _, fs := range a.Scorecard.Factors {
		fact("score_"+string(fs.Factor), fs.Value)
		s.Labels["peer_group_"+string(fs.Factor)] = fs.PeerGroup
	}
	for _, k := range slices.Sorted(maps.Keys(a.Indicators.Values)) {
		fact(k, a.Indicators.Values[k])
	}
	for _, fv := range a.FairValues {
		fact("fair_"+fv.Method+"_low", fv.Low)
		fact("fair_"+fv.Method+"_high", fv.High)
		for k, v := range fv.Assumptions {
			fact("fair_"+fv.Method+"_"+k, v)
		}
	}
	for _, sig := range append(slices.Clone(a.ValuationSignals), a.Timing.Signals...) {
		s.Labels["signal_"+sig.Code] = sig.Message
		for k, v := range sig.Data {
			if _, dup := s.Facts[k]; !dup {
				fact(k, v)
			}
		}
	}
	for i, n := range a.Notes {
		s.Labels[fmt.Sprintf("note_%d", i+1)] = n
	}

	want := BandClassFor(a.Asset.Class)
	for _, b := range o.Bands {
		if b.Class == want {
			fact("band_min_pct", b.Min)
			fact("band_max_pct", b.Max)
			s.Labels["band_class"] = b.Class
			s.Labels["band_lean"] = b.Lean
		}
	}
	for _, sig := range o.Signals {
		s.Labels["macro_"+sig.Code] = sig.Message
		for k, v := range sig.Data {
			fact(k, v)
		}
	}

	for i, n := range news {
		if i == maxHeadlines {
			break
		}
		s.Headlines = append(s.Headlines, domain.Headline{ID: fmt.Sprintf("news:%d", i+1), Title: n.Title, Source: n.Source, PublishedAt: n.PublishedAt})
	}
	return s
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
