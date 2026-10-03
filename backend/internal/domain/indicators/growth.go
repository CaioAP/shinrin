package indicators

import (
	"math"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Growth computes year-over-year growth of trailing-twelve-month revenue and
// net income, and the three-year revenue CAGR, as of asOf. Growth from a
// non-positive base is meaningless (a loss turning into a smaller loss is not
// "growth"), so those are left out.
func Growth(fin domain.Financials, asOf time.Time) map[string]float64 {
	out := make(map[string]float64)
	ttm := func(metric string, at time.Time) (float64, time.Time, bool) { return fin.TTM(metric, at) }

	rev, revEnd, ok := ttm(domain.MetricRevenue, asOf)
	if ok {
		if prev, ok := ttmYearsBefore(ttm, domain.MetricRevenue, revEnd, 1); ok && prev > 0 && rev > 0 {
			out[RevenueGrowth1Y] = rev/prev - 1
		}
		if prev, ok := ttmYearsBefore(ttm, domain.MetricRevenue, revEnd, 3); ok && prev > 0 && rev > 0 {
			out[RevenueCAGR3Y] = math.Pow(rev/prev, 1.0/3) - 1
		}
	}
	if ni, niEnd, ok := ttm(domain.MetricNetIncome, asOf); ok {
		if prev, ok := ttmYearsBefore(ttm, domain.MetricNetIncome, niEnd, 1); ok && prev > 0 && ni > 0 {
			out[EarningsGrowth1Y] = ni/prev - 1
		}
	}
	return out
}

// ttmYearsBefore returns the TTM value whose period ended exactly n years
// before end, so growth always compares like-for-like twelve-month windows.
func ttmYearsBefore(ttm func(string, time.Time) (float64, time.Time, bool), metric string, end time.Time, n int) (float64, bool) {
	target := monthEndYearsBefore(end, n)
	v, gotEnd, ok := ttm(metric, target)
	if !ok || gotEnd.Year() != target.Year() || gotEnd.Month() != target.Month() {
		return 0, false
	}
	return v, true
}

func monthEndYearsBefore(t time.Time, n int) time.Time {
	first := time.Date(t.Year()-n, t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.AddDate(0, 1, -1)
}
