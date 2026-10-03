package ingest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/ingest"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

var (
	petr  = domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}
	vale  = domain.AssetKey{Market: domain.MarketB3, Symbol: "VALE3"}
	mglu  = domain.AssetKey{Market: domain.MarketB3, Symbol: "MGLU3"}
	aapl  = domain.AssetKey{Market: domain.MarketUS, Symbol: "AAPL"}
	gone  = domain.AssetKey{Market: domain.MarketUS, Symbol: "GONE"}
	today = time.Date(2025, 3, 10, 22, 0, 0, 0, time.UTC)
)

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

type fixture struct {
	assets *memory.AssetRepository
	data   *memory.MarketDataStore
	svc    *ingest.Service
}

func newFixture(t *testing.T, assets ...domain.Asset) fixture {
	t.Helper()
	f := fixture{assets: memory.NewAssetRepository(), data: memory.NewMarketDataStore()}
	if err := f.assets.UpsertAssets(context.Background(), assets); err != nil {
		t.Fatal(err)
	}
	f.svc = ingest.New(ingest.Stores{Assets: f.assets, Prices: f.data, Fundamentals: f.data, Actions: f.data},
		ingest.Options{HistoryStart: d(2025, 1, 1), Now: func() time.Time { return today }})
	return f
}

func stock(k domain.AssetKey) domain.Asset {
	return domain.Asset{Key: k, Class: domain.ClassStock, Active: true, IndexMember: true}
}

// --- fakes -------------------------------------------------------------------

type fakeUniverse struct{ members []domain.Asset }

func (fakeUniverse) Name() string { return "fake" }
func (f fakeUniverse) Constituents(context.Context, domain.Index) ([]domain.Asset, error) {
	return f.members, nil
}

type fakeMarket struct {
	bars    []domain.PriceBar
	windows [][2]time.Time
}

func (*fakeMarket) Name() string          { return "fake-market" }
func (*fakeMarket) Market() domain.Market { return domain.MarketB3 }
func (f *fakeMarket) MarketPrices(_ context.Context, from, to time.Time) ([]domain.PriceBar, error) {
	f.windows = append(f.windows, [2]time.Time{from, to})
	var out []domain.PriceBar
	for _, b := range f.bars {
		if !b.Date.Before(from) && !b.Date.After(to) {
			out = append(out, b)
		}
	}
	return out, nil
}

type fakePrices struct {
	calls map[domain.AssetKey][2]time.Time
	fail  map[domain.AssetKey]error
}

func (*fakePrices) Name() string { return "fake-prices" }
func (f *fakePrices) DailyPrices(_ context.Context, k domain.AssetKey, from, to time.Time) ([]domain.PriceBar, error) {
	f.calls[k] = [2]time.Time{from, to}
	if err := f.fail[k]; err != nil {
		return nil, err
	}
	return []domain.PriceBar{{Asset: k, Date: to, Close: 1}}, nil
}
func (f *fakePrices) CorporateActions(_ context.Context, k domain.AssetKey, since time.Time) ([]domain.CorporateAction, error) {
	return []domain.CorporateAction{{Asset: k, ExDate: since, Type: domain.ActionDividend, Value: 0.1}}, nil
}

type fakeFundamentals struct{ since map[domain.AssetKey]time.Time }

func (fakeFundamentals) Name() string { return "fake-fundamentals" }
func (f fakeFundamentals) Fundamentals(_ context.Context, k domain.AssetKey, since time.Time) ([]domain.Fundamental, error) {
	f.since[k] = since
	return []domain.Fundamental{{Asset: k, PeriodEnd: d(2024, 12, 31), PeriodType: domain.PeriodFiscalYear, Metric: domain.MetricRevenue, Value: 1}}, nil
}

// --- tests -------------------------------------------------------------------

func TestSyncUniverse(t *testing.T) {
	ctx := context.Background()
	old := stock(mglu) // will leave the index
	kept := stock(vale)
	kept.CNPJ = "33.592.510/0001-54"
	f := newFixture(t, old, kept)

	src := fakeUniverse{members: []domain.Asset{
		{Key: petr, Class: domain.ClassStock, Name: "PETROBRAS"},
		{Key: vale, Class: domain.ClassStock, Name: "VALE"},
	}}
	if _, err := f.svc.SyncUniverse(ctx, src, domain.IndexIbovespa); err != nil {
		t.Fatal(err)
	}

	got, _ := f.assets.GetAsset(ctx, petr)
	if !got.IndexMember || !got.Active || got.Name != "PETROBRAS" {
		t.Errorf("new member = %+v", got)
	}
	got, _ = f.assets.GetAsset(ctx, vale)
	if got.CNPJ != kept.CNPJ {
		t.Errorf("identifier lost on refresh: %+v", got)
	}
	got, _ = f.assets.GetAsset(ctx, mglu)
	if got.IndexMember || !got.Active {
		t.Errorf("leaver should stay active but not a member: %+v", got)
	}

	if _, err := f.svc.SyncUniverse(ctx, fakeUniverse{}, domain.IndexIbovespa); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("empty constituents err = %v, want ErrInvalid", err)
	}
}

func TestSyncMarketPricesUsesEarliestGapAndDropsUntracked(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, stock(petr), stock(vale))
	_ = f.data.UpsertPrices(ctx, []domain.PriceBar{{Asset: petr, Date: d(2025, 3, 5), Close: 30}})
	_ = f.data.UpsertPrices(ctx, []domain.PriceBar{{Asset: vale, Date: d(2025, 3, 7), Close: 60}})

	src := &fakeMarket{bars: []domain.PriceBar{
		{Asset: petr, Date: d(2025, 3, 10), Close: 31},
		{Asset: vale, Date: d(2025, 3, 10), Close: 61},
		{Asset: mglu, Date: d(2025, 3, 10), Close: 9}, // not tracked
	}}
	r, err := f.svc.SyncMarketPrices(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(src.windows) != 1 || !src.windows[0][0].Equal(d(2025, 3, 6)) || !src.windows[0][1].Equal(d(2025, 3, 10)) {
		t.Errorf("windows = %v, want 2025-03-06..2025-03-10", src.windows)
	}
	if r.Rows != 2 {
		t.Errorf("rows = %d, want 2", r.Rows)
	}
	if _, ok, _ := f.data.LatestPriceDate(ctx, mglu); ok {
		t.Error("untracked ticker was stored")
	}
}

func TestSyncMarketPricesBackfillsAYearAtATime(t *testing.T) {
	f := newFixture(t, stock(petr))
	f.svc = ingest.New(ingest.Stores{Assets: f.assets, Prices: f.data, Fundamentals: f.data, Actions: f.data},
		ingest.Options{HistoryStart: d(2023, 6, 1), Now: func() time.Time { return today }})
	src := &fakeMarket{bars: []domain.PriceBar{
		{Asset: petr, Date: d(2023, 6, 1), Close: 1},
		{Asset: petr, Date: d(2024, 6, 3), Close: 2},
		{Asset: petr, Date: d(2025, 3, 10), Close: 3},
	}}
	r, err := f.svc.SyncMarketPrices(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]time.Time{{d(2023, 6, 1), d(2023, 12, 31)}, {d(2024, 1, 1), d(2024, 12, 31)}, {d(2025, 1, 1), d(2025, 3, 10)}}
	if len(src.windows) != len(want) {
		t.Fatalf("windows = %v", src.windows)
	}
	for i, w := range want {
		if !src.windows[i][0].Equal(w[0]) || !src.windows[i][1].Equal(w[1]) {
			t.Errorf("window %d = %v, want %v", i, src.windows[i], w)
		}
	}
	if r.Rows != 3 {
		t.Errorf("rows = %d", r.Rows)
	}
}

func TestSyncPricesIsIncrementalAndIsolatesFailures(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, stock(aapl), stock(gone), domain.Asset{Key: domain.AssetKey{Market: domain.MarketUS, Symbol: "BAD"}, Class: domain.ClassStock, Active: true})
	_ = f.data.UpsertPrices(ctx, []domain.PriceBar{{Asset: aapl, Date: d(2025, 3, 6), Close: 200}})

	bad := domain.AssetKey{Market: domain.MarketUS, Symbol: "BAD"}
	src := &fakePrices{
		calls: map[domain.AssetKey][2]time.Time{},
		fail:  map[domain.AssetKey]error{gone: domain.ErrNotFound, bad: errors.New("boom")},
	}
	r, err := f.svc.SyncPrices(ctx, domain.MarketUS, src, src)
	if err == nil {
		t.Fatal("want an error for the failing asset")
	}
	if r.Failed != 1 || r.Skipped != 1 || r.Rows != 2 { // AAPL: one bar + one action
		t.Errorf("result = %+v", r)
	}
	if w := src.calls[aapl]; !w[0].Equal(d(2025, 3, 7)) || !w[1].Equal(d(2025, 3, 10)) {
		t.Errorf("AAPL window = %v", w)
	}
	if w := src.calls[gone]; !w[0].Equal(d(2025, 1, 1)) {
		t.Errorf("new asset should start at HistoryStart, got %v", w)
	}
	acts, _ := f.data.CorporateActionsSince(ctx, aapl, time.Time{})
	if len(acts) != 1 {
		t.Errorf("actions stored = %d, want 1", len(acts))
	}
}

func TestSyncFundamentalsLookbackAndSkipsFIIs(t *testing.T) {
	ctx := context.Background()
	fii := domain.Asset{Key: domain.AssetKey{Market: domain.MarketB3, Symbol: "MXRF11"}, Class: domain.ClassFII, Active: true}
	f := newFixture(t, stock(petr), stock(vale), fii)
	_ = f.data.UpsertFundamentals(ctx, []domain.Fundamental{{Asset: petr, PeriodEnd: d(2024, 9, 30), PeriodType: domain.PeriodQuarter, Metric: domain.MetricRevenue}})

	src := fakeFundamentals{since: map[domain.AssetKey]time.Time{}}
	r, err := f.svc.SyncFundamentals(ctx, domain.MarketB3, src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped != 1 || r.Rows != 2 {
		t.Errorf("result = %+v", r)
	}
	if _, called := src.since[fii.Key]; called {
		t.Error("FII should not hit a company-filings source")
	}
	if got := src.since[vale]; !got.Equal(d(2025, 1, 1)) {
		t.Errorf("first sync since = %v", got)
	}
	if got := src.since[petr]; !got.Before(d(2024, 9, 30)) || got.Before(d(2023, 1, 1)) {
		t.Errorf("repeat sync since = %v, want about 18 months before 2024-09-30", got)
	}
}

func TestRoutinesHaveNames(t *testing.T) {
	f := newFixture(t)
	src := &fakePrices{calls: map[domain.AssetKey][2]time.Time{}}
	var rs []port.Routine = []port.Routine{
		f.svc.UniverseRoutine("u", fakeUniverse{}, domain.IndexSP500),
		f.svc.PricesRoutine("p", domain.MarketUS, src, nil),
	}
	for i, want := range []string{"u", "p"} {
		if rs[i].Name() != want {
			t.Errorf("routine %d name = %q", i, rs[i].Name())
		}
	}
	if err := rs[1].Run(context.Background()); err != nil {
		t.Errorf("empty market run: %v", err)
	}
}
