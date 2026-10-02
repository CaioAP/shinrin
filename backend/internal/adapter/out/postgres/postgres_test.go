package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/postgres"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// testStore migrates a throwaway schema in the database named by
// SHINRIN_TEST_DATABASE_URL (CI runs a Postgres service). Without it the
// tests are skipped, so `go test ./...` still works offline.
func testStore(t *testing.T) (*postgres.Store, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("SHINRIN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SHINRIN_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_%d", rand.Uint64())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})

	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	pool, err := postgres.Open(ctx, url+sep+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, pool); err != nil { // idempotent
		t.Fatal(err)
	}
	return postgres.New(pool), pool
}

var (
	petr = domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}
	aapl = domain.AssetKey{Market: domain.MarketUS, Symbol: "AAPL"}
)

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

func TestAssets(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	err := s.UpsertAssets(ctx, []domain.Asset{
		{Key: petr, Class: domain.ClassStock, Name: "PETROBRAS", IndexMember: true, Active: true},
		{Key: aapl, Class: domain.ClassStock, Name: "Apple", CIK: "0000320193", IndexMember: true, Active: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertAssets(ctx, []domain.Asset{{Key: petr, Class: domain.ClassStock, Name: "Petrobras PN", Active: true}}); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetAsset(ctx, petr)
	if err != nil || got.Name != "Petrobras PN" || got.IndexMember {
		t.Errorf("GetAsset = %+v, %v", got, err)
	}
	if _, err := s.GetAsset(ctx, domain.AssetKey{Market: domain.MarketUS, Symbol: "NOPE"}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("missing asset err = %v", err)
	}
	us, err := s.ListAssets(ctx, port.AssetFilter{Market: domain.MarketUS})
	if err != nil || len(us) != 1 || us[0].CIK != "0000320193" {
		t.Errorf("ListAssets(US) = %+v, %v", us, err)
	}
	yes := true
	members, _ := s.ListAssets(ctx, port.AssetFilter{IndexMember: &yes})
	if len(members) != 1 || members[0].Key != aapl {
		t.Errorf("members = %+v", members)
	}
	if err := s.Check(ctx); err != nil {
		t.Errorf("health: %v", err)
	}
}

func TestMarketData(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	if err := s.UpsertAssets(ctx, []domain.Asset{{Key: petr, Class: domain.ClassStock, Active: true}}); err != nil {
		t.Fatal(err)
	}

	// Prices: upsert replaces by date; unknown assets are ignored; duplicates
	// in one batch do not fail.
	bars := []domain.PriceBar{
		{Asset: petr, Date: d(2025, 3, 6), Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 10, Source: "t"},
		{Asset: petr, Date: d(2025, 3, 7), Open: 1, High: 2, Low: 0.5, Close: 1.6, AdjClose: 1.55, Volume: 11, Source: "t"},
		{Asset: petr, Date: d(2025, 3, 7), Open: 1, High: 2, Low: 0.5, Close: 1.6, AdjClose: 1.55, Volume: 11, Source: "t"},
		{Asset: aapl, Date: d(2025, 3, 7), Close: 200, Source: "t"},
	}
	if err := s.UpsertPrices(ctx, bars); err != nil {
		t.Fatal(err)
	}
	bars[0].Close = 1.4
	if err := s.UpsertPrices(ctx, bars[:1]); err != nil {
		t.Fatal(err)
	}
	got, err := s.PricesBetween(ctx, petr, d(2025, 3, 1), d(2025, 3, 31))
	if err != nil || len(got) != 2 || got[0].Close != 1.4 || got[0].AdjClose != 0 || got[1].AdjClose != 1.55 {
		t.Fatalf("PricesBetween = %+v, %v", got, err)
	}
	if last, ok, err := s.LatestPriceDate(ctx, petr); err != nil || !ok || !last.Equal(d(2025, 3, 7)) {
		t.Errorf("LatestPriceDate = %v, %v, %v", last, ok, err)
	}
	if _, ok, err := s.LatestPriceDate(ctx, aapl); err != nil || ok {
		t.Errorf("unknown asset latest = %v, %v", ok, err)
	}

	// Fundamentals.
	if err := s.UpsertFundamentals(ctx, []domain.Fundamental{
		{Asset: petr, PeriodEnd: d(2024, 12, 31), PeriodType: domain.PeriodFiscalYear, Metric: domain.MetricRevenue, Value: 490e9, Source: "cvm"},
		{Asset: petr, PeriodEnd: d(2025, 3, 31), PeriodType: domain.PeriodQuarter, Metric: domain.MetricRevenue, Value: 123e9, Source: "cvm"},
	}); err != nil {
		t.Fatal(err)
	}
	fs, err := s.FundamentalsSince(ctx, petr, d(2025, 1, 1))
	if err != nil || len(fs) != 1 || fs[0].Value != 123e9 || fs[0].PeriodType != domain.PeriodQuarter {
		t.Errorf("FundamentalsSince = %+v, %v", fs, err)
	}
	if end, ok, _ := s.LatestPeriodEnd(ctx, petr); !ok || !end.Equal(d(2025, 3, 31)) {
		t.Errorf("LatestPeriodEnd = %v %v", end, ok)
	}

	// Corporate actions.
	if err := s.UpsertCorporateActions(ctx, []domain.CorporateAction{
		{Asset: petr, ExDate: d(2024, 11, 22), Type: domain.ActionDividend, Value: 0.64, Source: "b3_api"},
		{Asset: petr, ExDate: d(2024, 11, 22), Type: domain.ActionJCP, Value: 0.2, Source: "b3_api"},
	}); err != nil {
		t.Fatal(err)
	}
	acts, err := s.CorporateActionsSince(ctx, petr, d(2024, 1, 1))
	if err != nil || len(acts) != 2 || acts[0].Type != domain.ActionDividend {
		t.Errorf("CorporateActionsSince = %+v, %v", acts, err)
	}

	// Indicators: a newer set wins, and re-writing a date replaces its keys.
	if err := s.UpsertIndicators(ctx, []domain.IndicatorSet{
		{Asset: petr, AsOf: d(2025, 3, 6), Values: map[string]float64{"pe": 4}},
		{Asset: petr, AsOf: d(2025, 3, 7), Values: map[string]float64{"pe": 4.1, "pb": 1.1}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertIndicators(ctx, []domain.IndicatorSet{{Asset: petr, AsOf: d(2025, 3, 7), Values: map[string]float64{"pb": 1.2}}}); err != nil {
		t.Fatal(err)
	}
	set, err := s.LatestIndicators(ctx, petr)
	if err != nil || !set.AsOf.Equal(d(2025, 3, 7)) || len(set.Values) != 1 || set.Values["pb"] != 1.2 {
		t.Errorf("LatestIndicators = %+v, %v", set, err)
	}
	if _, err := s.LatestIndicators(ctx, aapl); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("no indicators err = %v", err)
	}
}
