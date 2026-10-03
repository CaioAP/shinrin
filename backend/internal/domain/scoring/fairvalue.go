package scoring

import (
	"math"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
)

// Valuation-model assumptions. They are conventions, not forecasts, and are
// returned with every estimate so the UI can show them.
const (
	// GrahamMultiplier is Graham's 15x earnings times 1.5x book.
	GrahamMultiplier = 22.5
	// GrahamSafetyMargin: the low end of the range applies Graham's margin of
	// safety of one third.
	GrahamSafetyMargin = 1.0 / 3
	// Bazin's rule (popular in Brazil): a stock is worth buying while it
	// yields at least 6%; the low end asks for 8%.
	BazinMinYield    = 0.06
	BazinStrictYield = 0.08
	dcfYears         = 5
)

// DCFAssumptions are per-market discount-rate and growth conventions.
type DCFAssumptions struct {
	// EquityRiskPremium is added to the risk-free rate.
	EquityRiskPremium float64
	// TerminalGrowth is nominal perpetual growth after year five; Brazil's is
	// higher because its inflation target is.
	TerminalGrowth float64
}

// DCFDefaults per market.
var DCFDefaults = map[domain.Market]DCFAssumptions{
	domain.MarketUS: {EquityRiskPremium: 0.05, TerminalGrowth: 0.02},
	domain.MarketB3: {EquityRiskPremium: 0.06, TerminalGrowth: 0.04},
}

// FairValues estimates fair value ranges per share from an indicator
// snapshot. riskFree is the market's nominal risk-free rate in % a year
// (Selic for B3, the US 10-year yield for the US); ok false skips the DCF.
// Notes explain each model that could not run.
func FairValues(market domain.Market, ind map[string]float64, riskFree float64, hasRiskFree bool) ([]domain.FairValue, []string) {
	price := ind[indicators.Close]
	if price <= 0 {
		return nil, []string{"No price, so no fair value estimate."}
	}
	perShare := func(ratio string, invert bool) (float64, bool) {
		r, ok := ind[ratio]
		if !ok || r <= 0 {
			return 0, false
		}
		if invert {
			return price / r, true
		}
		return price * r, true
	}
	var out []domain.FairValue
	var notes []string

	eps, hasEPS := perShare(indicators.PE, true)
	bvps, hasBV := perShare(indicators.PB, true)
	if hasEPS && hasBV {
		g := math.Sqrt(GrahamMultiplier * eps * bvps)
		out = append(out, domain.FairValue{Method: "graham", Low: round2(g * (1 - GrahamSafetyMargin)), High: round2(g),
			Assumptions: map[string]float64{"eps": round2(eps), "book_value_per_share": round2(bvps), "multiplier": GrahamMultiplier}})
	} else {
		notes = append(notes, "Graham number needs positive earnings and book value.")
	}

	if dps, ok := perShare(indicators.DividendYield, false); ok {
		out = append(out, domain.FairValue{Method: "bazin", Low: round2(dps / BazinStrictYield), High: round2(dps / BazinMinYield),
			Assumptions: map[string]float64{"dividends_per_share_ttm": round2(dps), "min_yield": BazinMinYield, "strict_yield": BazinStrictYield}})
	} else {
		notes = append(notes, "Bazin needs dividends paid in the last twelve months.")
	}

	fcf, hasFCF := perShare(indicators.FCFYield, false)
	a, known := DCFDefaults[market]
	switch {
	case !hasFCF:
		notes = append(notes, "DCF needs positive free cash flow.")
	case !hasRiskFree || !known:
		notes = append(notes, "DCF skipped: no risk-free rate stored for this market yet.")
	default:
		growth := 0.0
		if g, ok := ind[indicators.RevenueCAGR3Y]; ok {
			growth = g
		} else if g, ok := ind[indicators.RevenueGrowth1Y]; ok {
			growth = g
		}
		growth = max(-0.05, min(0.15, growth))
		r := riskFree/100 + a.EquityRiskPremium
		lowR, highR := r+0.01, max(r-0.01, a.TerminalGrowth+0.02)
		lowG := max(-0.05, growth-0.03)
		out = append(out, domain.FairValue{Method: "dcf",
			Low:  round2(dcf(fcf, lowG, lowR, a.TerminalGrowth)),
			High: round2(dcf(fcf, growth, highR, a.TerminalGrowth)),
			Assumptions: map[string]float64{
				"fcf_per_share": round2(fcf), "growth_5y": round4(growth), "risk_free": round4(riskFree / 100),
				"equity_risk_premium": a.EquityRiskPremium, "discount_rate": round4(r), "terminal_growth": a.TerminalGrowth,
			}})
	}
	return out, notes
}

// dcf discounts five years of free cash flow growing at g, plus a Gordon
// terminal value, at rate r.
func dcf(fcf, g, r, terminal float64) float64 {
	v, cf := 0.0, fcf
	for t := 1; t <= dcfYears; t++ {
		cf *= 1 + g
		v += cf / math.Pow(1+r, float64(t))
	}
	tv := cf * (1 + terminal) / (r - terminal)
	return v + tv/math.Pow(1+r, dcfYears)
}

// ValuationViewOf combines the peer-relative valuation score with where the
// price sits against the fair value ranges.
func ValuationViewOf(sc domain.Scorecard, price float64, fvs []domain.FairValue) (domain.ValuationView, []domain.Signal) {
	var signals []domain.Signal
	total := 0.0
	if fs, ok := sc.Factor(domain.FactorValuation); ok {
		switch {
		case fs.Value >= 65:
			signals = append(signals, domain.Signal{Code: "cheap_vs_peers", Message: "Multiples are low against peers.", Weight: 1, Data: map[string]float64{"score_valuation": fs.Value}})
		case fs.Value <= 35:
			signals = append(signals, domain.Signal{Code: "expensive_vs_peers", Message: "Multiples are high against peers.", Weight: -1, Data: map[string]float64{"score_valuation": fs.Value}})
		}
	}
	if price > 0 && len(fvs) > 0 {
		var below, above int
		for _, fv := range fvs {
			if price < fv.Low {
				below++
			} else if price > fv.High {
				above++
			}
		}
		if w := float64(below-above) / float64(len(fvs)); w != 0 {
			code, msg := "below_fair_value", "Price is below the fair value range of most models."
			if w < 0 {
				code, msg = "above_fair_value", "Price is above the fair value range of most models."
			}
			signals = append(signals, domain.Signal{Code: code, Message: msg, Weight: round2(w),
				Data: map[string]float64{"price": price, "models_below": float64(below), "models_above": float64(above), "models": float64(len(fvs))}})
		}
	}
	for _, s := range signals {
		total += s.Weight
	}
	switch {
	case total >= 0.75:
		return domain.ValuationCheap, signals
	case total <= -0.75:
		return domain.ValuationExpensive, signals
	}
	return domain.ValuationFair, signals
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
