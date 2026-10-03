package indicators_test

import (
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
)

func ptr(v float64) *float64 { return &v }

func TestValuation(t *testing.T) {
	f := indicators.Fundamentals{
		RevenueTTM:           ptr(1000),
		EBITTTM:              ptr(200),
		DepreciationTTM:      ptr(50),
		NetIncomeTTM:         ptr(100),
		OperatingCashFlowTTM: ptr(180),
		CapexTTM:             ptr(80),
		Equity:               ptr(500),
		Cash:                 ptr(100),
		GrossDebt:            ptr(300),
		Shares:               ptr(100),
		DividendsPerShareTTM: ptr(0.6),
	}
	got := indicators.Valuation(10, f) // market cap 1000
	want := map[string]float64{
		indicators.MarketCap:       1000,
		indicators.PE:              10,
		indicators.PB:              2,
		indicators.PriceToSales:    1,
		indicators.EnterpriseValue: 1200,
		indicators.EVToEBITDA:      4.8,
		indicators.EVToEBIT:        6,
		indicators.DividendYield:   0.06,
		indicators.FCFYield:        0.1,
		indicators.ROE:             0.2,
		indicators.NetMargin:       0.1,
		indicators.EBITMargin:      0.2,
		indicators.NetDebtToEBITDA: 0.8,
	}
	for k, w := range want {
		if g, ok := got[k]; !ok || !near(g, w) {
			t.Errorf("%s = %v (present %v), want %v", k, g, ok, w)
		}
	}
}

func TestValuationSkipsMeaninglessRatios(t *testing.T) {
	got := indicators.Valuation(10, indicators.Fundamentals{
		NetIncomeTTM: ptr(-50), // a loss: P/E is meaningless
		Equity:       ptr(500),
		Shares:       ptr(100),
		EBITTTM:      ptr(20), // no depreciation: no EBITDA
		GrossDebt:    ptr(10),
	})
	for _, k := range []string{indicators.PE, indicators.EVToEBITDA, indicators.DividendYield} {
		if _, ok := got[k]; ok {
			t.Errorf("%s should be absent, got %v", k, got[k])
		}
	}
	if _, ok := got[indicators.EVToEBIT]; !ok {
		t.Error("EV/EBIT should be present")
	}
}

func TestSnapshotAdjustsRawPrices(t *testing.T) {
	key := domain.AssetKey{Market: domain.MarketB3, Symbol: "TEST3"}
	day := func(d int) time.Time { return time.Date(2025, 1, d, 0, 0, 0, 0, time.UTC) }
	bars := []domain.PriceBar{
		{Asset: key, Date: day(2), Close: 20},
		{Asset: key, Date: day(3), Close: 20},
		{Asset: key, Date: day(6), Close: 10}, // 2-for-1 split, not a 50% crash
	}
	actions := []domain.CorporateAction{{Asset: key, ExDate: day(6), Type: domain.ActionSplit, Value: 2}}
	set, ok := indicators.Snapshot(key, bars, nil, actions)
	if !ok {
		t.Fatal("no snapshot")
	}
	if d := set.Values[indicators.MaxDrawdown1Y]; d != 0 {
		t.Errorf("drawdown across a split = %v, want 0", d)
	}
	if c := set.Values[indicators.Close]; c != 10 || !set.AsOf.Equal(day(6)) {
		t.Errorf("close %v as of %v", c, set.AsOf)
	}
}

func TestFundamentalsAtDividends(t *testing.T) {
	asOf := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)
	actions := []domain.CorporateAction{
		{ExDate: asOf.AddDate(0, -2, 0), Type: domain.ActionDividend, Value: 1},
		{ExDate: asOf.AddDate(0, -6, 0), Type: domain.ActionJCP, Value: 0.5},
		{ExDate: asOf.AddDate(-2, 0, 0), Type: domain.ActionDividend, Value: 9}, // too old
		{ExDate: asOf.AddDate(0, -1, 0), Type: domain.ActionSplit, Value: 2},    // not cash
	}
	f := indicators.FundamentalsAt(domain.NewFinancials(nil), actions, asOf)
	if f.DividendsPerShareTTM == nil || *f.DividendsPerShareTTM != 1.5 {
		t.Fatalf("DPS = %v", f.DividendsPerShareTTM)
	}
}
