package domain

import (
	"cmp"
	"slices"
	"time"
)

// Standard fundamental metric names. Source adapters map their own line items
// (SEC XBRL tags, CVM account codes) onto these, so analysis code never knows
// where a number came from.
const (
	// Flow metrics: reported for a period (quarter or fiscal year).
	MetricRevenue           = "revenue"
	MetricEBIT              = "ebit"
	MetricDepreciation      = "depreciation_amortization"
	MetricNetIncome         = "net_income"
	MetricOperatingCashFlow = "operating_cash_flow"
	MetricCapex             = "capex" // positive number: cash spent
	MetricDividendsPaid     = "dividends_paid"

	// Stock metrics: a balance at the period end.
	MetricTotalAssets = "total_assets"
	MetricEquity      = "equity"
	MetricCash        = "cash" // cash, equivalents and short-term investments
	MetricGrossDebt   = "gross_debt"
	MetricShares      = "shares_outstanding" // excluding treasury shares
)

// Financials indexes one asset's reported fundamentals for point-in-time
// lookups. Build it with NewFinancials.
type Financials struct {
	byMetric map[string][]Fundamental // each slice sorted by PeriodEnd
}

// NewFinancials groups and sorts fundamentals. Rows for other assets are not
// filtered out, so pass one asset's rows only.
func NewFinancials(rows []Fundamental) Financials {
	f := Financials{byMetric: make(map[string][]Fundamental)}
	for _, r := range rows {
		f.byMetric[r.Metric] = append(f.byMetric[r.Metric], r)
	}
	for _, s := range f.byMetric {
		slices.SortFunc(s, func(a, b Fundamental) int { return a.PeriodEnd.Compare(b.PeriodEnd) })
	}
	return f
}

// Latest returns the most recent value of a metric whose period ended at or
// before asOf. Use it for balance-sheet (stock) metrics.
func (f Financials) Latest(metric string, asOf time.Time) (value float64, periodEnd time.Time, ok bool) {
	rows := f.byMetric[metric]
	for i := len(rows) - 1; i >= 0; i-- {
		if !rows[i].PeriodEnd.After(asOf) {
			return rows[i].Value, rows[i].PeriodEnd, true
		}
	}
	return 0, time.Time{}, false
}

// TTM returns the trailing-twelve-months sum of a flow metric as of asOf.
//
// Companies do not file a fourth quarter: the annual report covers it. So the
// fourth quarter is derived as the fiscal-year value minus the three quarters
// before it. When four consecutive quarters are not available, the latest
// fiscal year is used if it is the newest period on file.
func (f Financials) TTM(metric string, asOf time.Time) (value float64, periodEnd time.Time, ok bool) {
	quarters := make(map[int]float64) // month index of period end -> value
	var years []Fundamental
	for _, r := range f.byMetric[metric] {
		if r.PeriodEnd.After(asOf) {
			continue
		}
		switch r.PeriodType {
		case PeriodQuarter:
			quarters[monthIndex(r.PeriodEnd)] = r.Value
		case PeriodFiscalYear:
			years = append(years, r)
		}
	}
	for _, y := range years {
		end := monthIndex(y.PeriodEnd)
		if _, have := quarters[end]; have {
			continue
		}
		q1, ok1 := quarters[end-9]
		q2, ok2 := quarters[end-6]
		q3, ok3 := quarters[end-3]
		if ok1 && ok2 && ok3 {
			quarters[end] = y.Value - q1 - q2 - q3
		}
	}

	latestQ := -1
	for m := range quarters {
		latestQ = max(latestQ, m)
	}
	var latestFY *Fundamental
	if len(years) > 0 {
		latestFY = &years[len(years)-1]
	}

	if latestQ >= 0 {
		sum, complete := 0.0, true
		for _, m := range []int{latestQ, latestQ - 3, latestQ - 6, latestQ - 9} {
			v, have := quarters[m]
			if !have {
				complete = false
				break
			}
			sum += v
		}
		if complete {
			return sum, monthEnd(latestQ), true
		}
	}
	if latestFY != nil && monthIndex(latestFY.PeriodEnd) >= latestQ {
		return latestFY.Value, latestFY.PeriodEnd, true
	}
	return 0, time.Time{}, false
}

// Metrics lists the metric names on file, sorted.
func (f Financials) Metrics() []string {
	out := make([]string, 0, len(f.byMetric))
	for m := range f.byMetric {
		out = append(out, m)
	}
	slices.SortFunc(out, cmp.Compare[string])
	return out
}

func monthIndex(t time.Time) int { return t.Year()*12 + int(t.Month()) - 1 }

func monthEnd(idx int) time.Time {
	y, m := idx/12, time.Month(idx%12+1)
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC)
}
