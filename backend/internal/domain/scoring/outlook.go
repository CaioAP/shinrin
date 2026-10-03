package scoring

import (
	"slices"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Allocation classes used by the bands.
const (
	ClassFixedIncome = "fixed_income"
	ClassStocks      = "stocks"
	ClassRealEstate  = "real_estate"
)

// Bands are each profile's allocation ranges in percent, modelled on the
// suitability process Brazilian brokers use.
var Bands = map[domain.RiskProfile][]domain.AllocationBand{
	domain.ProfileConservative: {{Class: ClassFixedIncome, Min: 60, Max: 80}, {Class: ClassStocks, Min: 10, Max: 30}, {Class: ClassRealEstate, Min: 5, Max: 15}},
	domain.ProfileModerate:     {{Class: ClassFixedIncome, Min: 35, Max: 55}, {Class: ClassStocks, Min: 30, Max: 50}, {Class: ClassRealEstate, Min: 5, Max: 20}},
	domain.ProfileAggressive:   {{Class: ClassFixedIncome, Min: 10, Max: 30}, {Class: ClassStocks, Min: 50, Max: 75}, {Class: ClassRealEstate, Min: 5, Max: 20}},
}

// Macro thresholds for the tilts.
const (
	HighRealRate = 6.0 // % a year: post-fixed bonds pay well with little risk
	LowRealRate  = 3.0
)

// MacroContext is the latest macro data the outlook reads. A nil field was
// not available (for example, Brazilian series before macro_br has run).
type MacroContext struct {
	AsOf    time.Time
	Selic   *float64 // % a year
	IPCA12M *float64 // % over the last twelve months
	UST10Y  *float64 // % a year
	UST3M   *float64 // % a year
}

// OutlookFor returns a profile's allocation bands, each with a lean (low,
// mid, high) from macro conditions: high real interest in Brazil favours
// post-fixed bonds over stocks and FIIs, and an inverted US yield curve, a
// classic recession warning, calls for caution on stocks.
func OutlookFor(p domain.RiskProfile, m MacroContext) domain.Outlook {
	o := domain.Outlook{Profile: p, AsOf: m.AsOf, Bands: slices.Clone(Bands[p])}
	lean := map[string]int{} // -1 low, 0 mid, +1 high
	if m.Selic != nil && m.IPCA12M != nil {
		real := ((1+*m.Selic/100)/(1+*m.IPCA12M/100) - 1) * 100
		data := map[string]float64{"selic": *m.Selic, "ipca_12m": round2(*m.IPCA12M), "real_rate_br": round2(real)}
		switch {
		case real >= HighRealRate:
			o.Signals = append(o.Signals, domain.Signal{Code: "high_real_rate_br", Message: "Brazil's real interest rate is high: post-fixed bonds pay well for little risk.", Weight: -1, Data: data})
			lean[ClassFixedIncome]++
			lean[ClassStocks]--
			lean[ClassRealEstate]--
		case real <= LowRealRate:
			o.Signals = append(o.Signals, domain.Signal{Code: "low_real_rate_br", Message: "Brazil's real interest rate is low: fixed income pays little above inflation.", Weight: 1, Data: data})
			lean[ClassFixedIncome]--
			lean[ClassStocks]++
			lean[ClassRealEstate]++
		}
	} else {
		o.Notes = append(o.Notes, "Selic or IPCA is not stored yet, so Brazilian rate signals were skipped.")
	}
	if m.UST10Y != nil && m.UST3M != nil {
		spread := *m.UST10Y - *m.UST3M
		data := map[string]float64{"ust_10y": *m.UST10Y, "ust_3m": *m.UST3M, "spread": round2(spread)}
		if spread < 0 {
			o.Signals = append(o.Signals, domain.Signal{Code: "us_curve_inverted", Message: "The US yield curve is inverted (10-year below 3-month), a classic recession warning.", Weight: -1, Data: data})
			lean[ClassStocks]--
		} else {
			o.Signals = append(o.Signals, domain.Signal{Code: "us_curve_normal", Message: "The US yield curve is upward sloping.", Weight: 0, Data: data})
		}
	} else {
		o.Notes = append(o.Notes, "US Treasury yields are not stored yet, so the yield curve signal was skipped.")
	}
	for i := range o.Bands {
		switch l := lean[o.Bands[i].Class]; {
		case l > 0:
			o.Bands[i].Lean = "high"
		case l < 0:
			o.Bands[i].Lean = "low"
		default:
			o.Bands[i].Lean = "mid"
		}
	}
	return o
}

// Compound12M compounds the last twelve monthly percentage changes (IPCA is
// published as a monthly change) into a twelve-month change in percent.
// points must be sorted oldest first; ok is false with fewer than twelve.
func Compound12M(points []domain.MacroPoint) (float64, bool) {
	if len(points) < 12 {
		return 0, false
	}
	acc := 1.0
	for _, p := range points[len(points)-12:] {
		acc *= 1 + p.Value/100
	}
	return (acc - 1) * 100, true
}
