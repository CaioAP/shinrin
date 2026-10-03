package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func TestScoresAndReports(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	vale := domain.AssetKey{Market: domain.MarketB3, Symbol: "VALE3"}
	if err := s.UpsertAssets(ctx, []domain.Asset{
		{Key: petr, Class: domain.ClassStock, Active: true}, {Key: vale, Class: domain.ClassStock, Active: true},
	}); err != nil {
		t.Fatal(err)
	}

	val := domain.FactorScore{Factor: domain.FactorValuation, Value: 88.5, PeerGroup: "B3 stock, sector Energy",
		Inputs: []domain.ScoreInput{{Metric: "pe", Value: 4.1, Points: 100, Weight: 1, Peers: 7}}}
	mom := domain.FactorScore{Factor: domain.FactorMomentum, Value: 40}
	if err := s.UpsertScorecards(ctx, []domain.Scorecard{
		{Asset: petr, AsOf: d(2026, 10, 1), Factors: []domain.FactorScore{mom, val}},
		{Asset: petr, AsOf: d(2026, 10, 2), Factors: []domain.FactorScore{mom, val, {Factor: domain.FactorQuality, Value: 70}}},
		{Asset: vale, AsOf: d(2026, 10, 2), Factors: []domain.FactorScore{{Factor: domain.FactorRisk, Value: 60}}},
	}); err != nil {
		t.Fatal(err)
	}
	// Re-scoring a date replaces its factors: quality is gone.
	if err := s.UpsertScorecards(ctx, []domain.Scorecard{{Asset: petr, AsOf: d(2026, 10, 2), Factors: []domain.FactorScore{mom, val}}}); err != nil {
		t.Fatal(err)
	}

	sc, err := s.LatestScorecard(ctx, petr)
	if err != nil || !sc.AsOf.Equal(d(2026, 10, 2)) || len(sc.Factors) != 2 {
		t.Fatalf("LatestScorecard = %+v, %v", sc, err)
	}
	got := sc.Factors[0] // valuation sorts before momentum
	if got.Factor != domain.FactorValuation || got.PeerGroup != val.PeerGroup || len(got.Inputs) != 1 || got.Inputs[0] != val.Inputs[0] {
		t.Errorf("valuation = %+v", got)
	}
	all, err := s.LatestScorecards(ctx, domain.MarketB3)
	if err != nil || len(all) != 2 || all[0].Asset != petr || all[1].Asset != vale {
		t.Errorf("LatestScorecards = %+v, %v", all, err)
	}
	if _, err := s.LatestScorecard(ctx, aapl); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unscored err = %v", err)
	}

	id, err := s.SaveReport(ctx, domain.Report{
		Kind: domain.ReportKindAsset, Asset: petr, Profile: domain.ProfileModerate, AsOf: d(2026, 10, 2),
		Snapshot: domain.ReportSnapshot{Facts: map[string]float64{"pe": 4.1}, Labels: map[string]string{"symbol": "PETR4"},
			Headlines: []domain.Headline{{ID: "news:1", Title: "t"}}},
		Output:   domain.ReportOutput{Summary: "s", AllocationMaxPct: 5, CitedData: []string{"pe"}},
		Omitted:  []string{"bull_case[1]"},
		Provider: "anthropic", Model: "claude-opus-5-5", TokensIn: 10, TokensOut: 5,
	})
	if err != nil || id == 0 {
		t.Fatalf("SaveReport = %d, %v", id, err)
	}
	var max float64
	var omitted []string
	if err := pool.QueryRow(ctx, `SELECT (output->'suggested_allocation_pct'->>'max')::float8, omitted FROM ai_reports WHERE id = $1`, id).Scan(&max, &omitted); err != nil || max != 5 || omitted[0] != "bull_case[1]" {
		t.Errorf("stored report = %v %v %v", max, omitted, err)
	}
}
