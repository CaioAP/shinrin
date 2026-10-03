package analysis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/analysis"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

var (
	asOf = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	petr = domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}
	vale = domain.AssetKey{Market: domain.MarketB3, Symbol: "VALE3"}
	mxrf = domain.AssetKey{Market: domain.MarketB3, Symbol: "MXRF11"}
)

func setup(t *testing.T) (*analysis.Service, *memory.FeedStore) {
	t.Helper()
	ctx := context.Background()
	assets, data, feeds, scores := memory.NewAssetRepository(), memory.NewMarketDataStore(), memory.NewFeedStore(), memory.NewAnalysisStore()
	_ = assets.UpsertAssets(ctx, []domain.Asset{
		{Key: petr, Class: domain.ClassStock, Name: "Petrobras", Active: true},
		{Key: vale, Class: domain.ClassStock, Name: "Vale", Active: true},
		{Key: mxrf, Class: domain.ClassFII, Name: "Maxi Renda", Active: true},
	})
	_ = data.UpsertIndicators(ctx, []domain.IndicatorSet{{Asset: petr, AsOf: asOf, Values: map[string]float64{
		indicators.Close: 40, indicators.PE: 4, indicators.PB: 1, indicators.DividendYield: 0.12, indicators.FCFYield: 0.15,
		indicators.SMA50: 38, indicators.SMA200: 35, indicators.MACDHistogram: 0.1, indicators.RSI14: 60,
	}}})
	_ = scores.UpsertScorecards(ctx, []domain.Scorecard{
		{Asset: petr, AsOf: asOf, Factors: []domain.FactorScore{{Factor: domain.FactorValuation, Value: 90}, {Factor: domain.FactorIncome, Value: 80}, {Factor: domain.FactorGrowth, Value: 20}}},
		{Asset: vale, AsOf: asOf, Factors: []domain.FactorScore{{Factor: domain.FactorValuation, Value: 50}, {Factor: domain.FactorGrowth, Value: 90}}},
		{Asset: mxrf, AsOf: asOf, Factors: []domain.FactorScore{{Factor: domain.FactorIncome, Value: 70}}},
	})
	svc := analysis.New(analysis.Stores{Assets: assets, Scores: scores, Indicators: data, Macro: feeds}, func() time.Time { return asOf.Add(24 * time.Hour) })
	return svc, feeds
}

func TestAnalyze(t *testing.T) {
	ctx := context.Background()
	svc, feeds := setup(t)

	a, err := svc.Analyze(ctx, petr, domain.ProfileConservative)
	if err != nil {
		t.Fatal(err)
	}
	// Conservative: valuation .20, income .20, growth .05 -> (18+16+1)/.45.
	if a.Composite != 77.8 || a.Price != 40 {
		t.Errorf("composite = %v, price = %v", a.Composite, a.Price)
	}
	if len(a.FairValues) != 2 || len(a.Notes) != 1 {
		t.Errorf("without Selic the DCF is skipped with a note: %+v %v", a.FairValues, a.Notes)
	}
	if a.Valuation != domain.ValuationCheap || a.Timing.View != domain.TimingAccumulate {
		t.Errorf("views = %v / %+v", a.Valuation, a.Timing)
	}

	_ = feeds.UpsertMacro(ctx, []domain.MacroPoint{{Series: domain.SeriesSelicTarget, Date: asOf, Value: 15}})
	a, _ = svc.Analyze(ctx, petr, domain.ProfileConservative)
	if len(a.FairValues) != 3 {
		t.Errorf("with Selic the DCF runs: %+v", a.FairValues)
	}

	if _, err := svc.Analyze(ctx, domain.AssetKey{Market: domain.MarketB3, Symbol: "NOPE3"}, domain.ProfileModerate); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown asset err = %v", err)
	}
	f, err := svc.Analyze(ctx, mxrf, domain.ProfileModerate)
	if err != nil || len(f.Notes) == 0 || f.Indicators.Values == nil {
		t.Errorf("fund without indicators = %+v, %v", f, err)
	}
}

func TestRank(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	got, err := svc.Rank(ctx, port.RankFilter{Market: domain.MarketB3, Class: domain.ClassStock, Profile: domain.ProfileAggressive})
	if err != nil || len(got) != 2 {
		t.Fatalf("Rank = %+v, %v", got, err)
	}
	// Aggressive weights growth .25: VALE (50*.15+90*.25)/.40 = 75 beats PETR (13.5+4+5)/.45 = 50.
	if got[0].Asset.Key != vale || got[0].Composite != 75 || got[1].Composite != 50 {
		t.Errorf("aggressive order = %+v", got)
	}
	got, _ = svc.Rank(ctx, port.RankFilter{Profile: domain.ProfileConservative, Limit: 1})
	if len(got) != 1 || got[0].Asset.Key != petr {
		t.Errorf("conservative top = %+v", got)
	}
}

func TestOutlook(t *testing.T) {
	ctx := context.Background()
	svc, feeds := setup(t)
	o, err := svc.Outlook(ctx, domain.ProfileModerate)
	if err != nil || len(o.Notes) != 2 {
		t.Fatalf("empty macro outlook = %+v, %v", o, err)
	}
	var pts []domain.MacroPoint
	for i := range 12 {
		pts = append(pts, domain.MacroPoint{Series: domain.SeriesIPCA, Date: time.Date(2025, time.Month(10+i), 1, 0, 0, 0, 0, time.UTC), Value: 0.4})
	}
	pts = append(pts, domain.MacroPoint{Series: domain.SeriesSelicTarget, Date: asOf, Value: 15})
	_ = feeds.UpsertMacro(ctx, pts)
	o, _ = svc.Outlook(ctx, domain.ProfileModerate)
	if len(o.Signals) != 1 || o.Signals[0].Code != "high_real_rate_br" || !o.AsOf.Equal(asOf) {
		t.Errorf("outlook = %+v", o)
	}
}
