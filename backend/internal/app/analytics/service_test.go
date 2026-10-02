package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/analytics"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
)

func TestComputeMarket(t *testing.T) {
	ctx := context.Background()
	key := domain.AssetKey{Market: domain.MarketB3, Symbol: "ITUB4"}
	assets := memory.NewAssetRepository()
	data := memory.NewMarketDataStore()
	_ = assets.UpsertAssets(ctx, []domain.Asset{
		{Key: key, Class: domain.ClassStock, Active: true},
		{Key: domain.AssetKey{Market: domain.MarketB3, Symbol: "NOBAR3"}, Class: domain.ClassStock, Active: true},
	})

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var bars []domain.PriceBar
	for i := range 300 {
		bars = append(bars, domain.PriceBar{Asset: key, Date: start.AddDate(0, 0, i), Close: 10 + float64(i)/100})
	}
	_ = data.UpsertPrices(ctx, bars)
	last := bars[len(bars)-1]
	_ = data.UpsertFundamentals(ctx, []domain.Fundamental{
		{Asset: key, PeriodEnd: time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC), PeriodType: domain.PeriodFiscalYear, Metric: domain.MetricNetIncome, Value: 1000},
		{Asset: key, PeriodEnd: time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC), PeriodType: domain.PeriodFiscalYear, Metric: domain.MetricShares, Value: 1000},
	})

	svc := analytics.New(analytics.Stores{Assets: assets, Prices: data, Fundamentals: data, Actions: data, Indicators: data},
		func() time.Time { return last.Date.Add(20 * time.Hour) }, nil)
	n, err := svc.ComputeMarket(ctx, domain.MarketB3)
	if err != nil || n != 1 {
		t.Fatalf("ComputeMarket = %d, %v; want 1 snapshot", n, err)
	}
	unit := domain.AssetKey{Market: domain.MarketB3, Symbol: "TAEE11"}
	_ = assets.UpsertAssets(ctx, []domain.Asset{{Key: unit, Class: domain.ClassStock, Active: true}})
	for _, b := range bars {
		b.Asset = unit
		_ = data.UpsertPrices(ctx, []domain.PriceBar{b})
	}
	_ = data.UpsertFundamentals(ctx, []domain.Fundamental{
		{Asset: unit, PeriodEnd: time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC), PeriodType: domain.PeriodFiscalYear, Metric: domain.MetricNetIncome, Value: 1000},
		{Asset: unit, PeriodEnd: time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC), PeriodType: domain.PeriodFiscalYear, Metric: domain.MetricShares, Value: 1000},
	})
	if _, err := svc.ComputeMarket(ctx, domain.MarketB3); err != nil {
		t.Fatal(err)
	}
	if u, _ := data.LatestIndicators(ctx, unit); u.Values[indicators.PE] != 0 || u.Values[indicators.Close] == 0 {
		t.Errorf("unit should have technicals but no per-share ratios: %v", u.Values)
	}

	set, err := data.LatestIndicators(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if !set.AsOf.Equal(last.Date) {
		t.Errorf("as of %v, want %v", set.AsOf, last.Date)
	}
	if pe := set.Values[indicators.PE]; pe < last.Close-1e-9 || pe > last.Close+1e-9 { // mcap = close*1000, NI = 1000
		t.Errorf("P/E = %v, want %v", pe, last.Close)
	}
	for _, k := range []string{indicators.SMA200, indicators.RSI14, indicators.Return12M} {
		if _, ok := set.Values[k]; !ok {
			t.Errorf("%s missing", k)
		}
	}
}
