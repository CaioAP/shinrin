package scoring

import (
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
)

// Metrics the engine derives on top of domain/indicators. Ratios are
// fractions, like the indicators.
const (
	PriceVsSMA200 = "price_vs_sma_200" // close / 200-day average - 1
	PriceVsSMA50  = "price_vs_sma_50"
	FromHigh52W   = "from_high_52w" // close / 52-week high - 1 (zero or negative)
	FromLow52W    = "from_low_52w"  // close / 52-week low - 1 (zero or positive)
	PayoutRatio   = "payout_ratio"  // dividend yield x P/E = dividends / earnings
	DividendYears = "dividend_years_5y"
	NewsTone      = "news_tone_30d" // mean headline tone, -1 to 1
	NewsCount     = "news_count_30d"
)

// Metrics merges an indicator snapshot with extra inputs computed elsewhere
// (dividend history, news tone) and adds the derived ratios.
func Metrics(ind map[string]float64, extra map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(ind)+len(extra)+6)
	for k, v := range ind {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	c, hasClose := ind[indicators.Close]
	if hasClose && c > 0 {
		ratio := func(name, base string) {
			if b, ok := ind[base]; ok && b > 0 {
				out[name] = c/b - 1
			}
		}
		ratio(PriceVsSMA200, indicators.SMA200)
		ratio(PriceVsSMA50, indicators.SMA50)
		ratio(FromHigh52W, indicators.High52W)
		ratio(FromLow52W, indicators.Low52W)
	}
	if dy, ok := ind[indicators.DividendYield]; ok {
		if pe, ok := ind[indicators.PE]; ok && pe > 0 && dy > 0 {
			out[PayoutRatio] = dy * pe
		}
	}
	return out
}

// CountDividendYears counts the calendar years, among the five before asOf's
// year plus asOf's own, in which the asset paid cash (dividends or JCP),
// capped at five. Consistency matters to income investors more than one
// large payment.
func CountDividendYears(actions []domain.CorporateAction, asOf time.Time) float64 {
	from := asOf.AddDate(-5, 0, 0)
	years := map[int]bool{}
	for _, a := range actions {
		if a.IsCash() && a.Value > 0 && a.ExDate.After(from) && !a.ExDate.After(asOf) {
			years[a.ExDate.Year()] = true
		}
	}
	return float64(min(len(years), 5))
}
