package scoring_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/scoring"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
	engine "github.com/CaioAP/shinrin/backend/internal/domain/scoring"
)

func TestScoreMarket(t *testing.T) {
	ctx := context.Background()
	asOf := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	assets, data, feeds, scores := memory.NewAssetRepository(), memory.NewMarketDataStore(), memory.NewFeedStore(), memory.NewAnalysisStore()

	var list []domain.Asset
	for i := range 4 {
		k := domain.AssetKey{Market: domain.MarketB3, Symbol: domain.Symbol(fmt.Sprintf("STK%d", i+1))}
		list = append(list, domain.Asset{Key: k, Class: domain.ClassStock, Active: true})
		_ = data.UpsertIndicators(ctx, []domain.IndicatorSet{{Asset: k, AsOf: asOf, Values: map[string]float64{
			indicators.Close: 10, indicators.PE: float64(5 + i), indicators.DividendYield: 0.05,
		}}})
	}
	noData := domain.AssetKey{Market: domain.MarketB3, Symbol: "NEW3"}
	bond := domain.AssetKey{Market: domain.MarketB3, Symbol: "LTN-20290101"}
	list = append(list, domain.Asset{Key: noData, Class: domain.ClassStock, Active: true}, domain.Asset{Key: bond, Class: domain.ClassGovBond, Active: true})
	_ = assets.UpsertAssets(ctx, list)

	stk1 := list[0].Key
	_ = data.UpsertCorporateActions(ctx, []domain.CorporateAction{
		{Asset: stk1, ExDate: asOf.AddDate(-1, 0, 0), Type: domain.ActionDividend, Value: 0.5},
		{Asset: stk1, ExDate: asOf.AddDate(-2, 0, 0), Type: domain.ActionJCP, Value: 0.5},
	})
	_ = feeds.UpsertNews(ctx, []domain.NewsItem{
		{URL: "u1", Title: "STK1 posts record profit", PublishedAt: asOf.AddDate(0, 0, -3), Assets: []domain.AssetKey{stk1}},
		{URL: "u2", Title: "Old lawsuit", PublishedAt: asOf.AddDate(0, -3, 0), Assets: []domain.AssetKey{stk1}},
	})

	svc := scoring.New(scoring.Stores{Assets: assets, Indicators: data, Actions: data, News: feeds, Scores: scores},
		func() time.Time { return asOf.Add(20 * time.Hour) }, nil)
	if err := svc.Routine("scoring", domain.MarketB3).Run(ctx); err != nil {
		t.Fatalf("routine: %v", err)
	}
	cards, _ := scores.LatestScorecards(ctx, domain.MarketB3)
	if len(cards) != 4 {
		t.Fatalf("scored %d assets, want 4 (no indicators and bonds skipped)", len(cards))
	}
	sc, _ := scores.LatestScorecard(ctx, stk1)
	v, _ := sc.Factor(domain.FactorValuation)
	// Lowest P/E of four (100 points, weight 1); dividend yields all equal
	// (50 points, weight 0.5).
	if want := (100*1 + 50*0.5) / 1.5; v.Value != float64(int(want*10+0.5))/10 {
		t.Errorf("STK1 valuation = %+v, want %.1f", v, want)
	}
	inc, _ := sc.Factor(domain.FactorIncome)
	var years float64
	for _, in := range inc.Inputs {
		if in.Metric == engine.DividendYears {
			years = in.Value
		}
	}
	if years != 2 {
		t.Errorf("dividend years = %v, want 2 (%+v)", years, inc)
	}
	sent, ok := sc.Factor(domain.FactorSentiment)
	if !ok || sent.Value != 100 {
		t.Errorf("sentiment = %+v, %v; want 100 from the one recent positive headline", sent, ok)
	}
}
