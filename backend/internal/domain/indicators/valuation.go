package indicators

import (
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Fundamentals is what the valuation ratios need, already reduced to single
// numbers (TTM sums for flows, latest balances for stocks). Build it with
// FundamentalsAt; a nil field means the value is unknown.
type Fundamentals struct {
	RevenueTTM, EBITTTM, DepreciationTTM, NetIncomeTTM *float64
	OperatingCashFlowTTM, CapexTTM                     *float64
	Equity, Cash, GrossDebt, Shares                    *float64
	// DividendsPerShareTTM sums cash dividends and JCP with ex-dates in the
	// last twelve months.
	DividendsPerShareTTM *float64
}

// FundamentalsAt reduces reported fundamentals and corporate actions to the
// numbers valid on asOf.
func FundamentalsAt(fin domain.Financials, actions []domain.CorporateAction, asOf time.Time) Fundamentals {
	ttm := func(m string) *float64 {
		if v, _, ok := fin.TTM(m, asOf); ok {
			return &v
		}
		return nil
	}
	latest := func(m string) *float64 {
		if v, _, ok := fin.Latest(m, asOf); ok {
			return &v
		}
		return nil
	}
	f := Fundamentals{
		RevenueTTM:           ttm(domain.MetricRevenue),
		EBITTTM:              ttm(domain.MetricEBIT),
		DepreciationTTM:      ttm(domain.MetricDepreciation),
		NetIncomeTTM:         ttm(domain.MetricNetIncome),
		OperatingCashFlowTTM: ttm(domain.MetricOperatingCashFlow),
		CapexTTM:             ttm(domain.MetricCapex),
		Equity:               latest(domain.MetricEquity),
		Cash:                 latest(domain.MetricCash),
		GrossDebt:            latest(domain.MetricGrossDebt),
		Shares:               latest(domain.MetricShares),
	}
	yearAgo := asOf.AddDate(-1, 0, 0)
	dps, seen := 0.0, false
	for _, a := range actions {
		if a.IsCash() && a.ExDate.After(yearAgo) && !a.ExDate.After(asOf) {
			dps += a.Value
			seen = true
		}
	}
	if seen {
		f.DividendsPerShareTTM = &dps
	}
	return f
}

// Valuation computes valuation, quality and leverage ratios at price. Ratios
// whose inputs are missing, or whose denominator is not positive, are left
// out of the map.
func Valuation(price float64, f Fundamentals) map[string]float64 {
	out := make(map[string]float64)
	set := func(name string, num float64, den *float64) {
		if den != nil && *den > 0 {
			out[name] = num / *den
		}
	}
	if f.DividendsPerShareTTM != nil && price > 0 {
		out[DividendYield] = *f.DividendsPerShareTTM / price
	}
	if f.NetIncomeTTM != nil && f.Equity != nil && *f.Equity > 0 {
		out[ROE] = *f.NetIncomeTTM / *f.Equity
	}
	if f.RevenueTTM != nil && *f.RevenueTTM > 0 {
		if f.NetIncomeTTM != nil {
			out[NetMargin] = *f.NetIncomeTTM / *f.RevenueTTM
		}
		if f.EBITTTM != nil {
			out[EBITMargin] = *f.EBITTTM / *f.RevenueTTM
		}
	}

	ebitda := sum(f.EBITTTM, f.DepreciationTTM)
	var netDebt *float64
	if f.GrossDebt != nil {
		nd := *f.GrossDebt
		if f.Cash != nil {
			nd -= *f.Cash
		}
		netDebt = &nd
		if ebitda != nil && *ebitda > 0 {
			out[NetDebtToEBITDA] = nd / *ebitda
		}
	}

	if f.Shares == nil || *f.Shares <= 0 || price <= 0 {
		return out
	}
	mcap := price * *f.Shares
	out[MarketCap] = mcap
	set(PE, mcap, f.NetIncomeTTM)
	set(PB, mcap, f.Equity)
	set(PriceToSales, mcap, f.RevenueTTM)
	if f.OperatingCashFlowTTM != nil && f.CapexTTM != nil {
		out[FCFYield] = (*f.OperatingCashFlowTTM - *f.CapexTTM) / mcap
	}
	if netDebt != nil {
		ev := mcap + *netDebt
		out[EnterpriseValue] = ev
		set(EVToEBITDA, ev, ebitda)
		set(EVToEBIT, ev, f.EBITTTM)
	}
	return out
}

// sum returns a+b, or nil if either is unknown. EBITDA needs both parts; an
// EBIT-only figure would flatter EV/EBITDA.
func sum(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a + *b
	return &v
}
