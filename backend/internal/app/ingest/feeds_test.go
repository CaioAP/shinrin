package ingest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

type fakeQuotes struct {
	asked []domain.AssetKey
	err   error
}

func (*fakeQuotes) Name() string { return "fake-quotes" }
func (f *fakeQuotes) Quotes(_ context.Context, keys []domain.AssetKey) ([]domain.Quote, error) {
	f.asked = keys
	// Answer only the first key, as a source that stops part way would.
	return []domain.Quote{{Asset: keys[0], Price: 10, AsOf: today, Source: "fake"}}, f.err
}

func TestSyncQuotesKeepsPartialResultsAndSkipsBonds(t *testing.T) {
	bond := domain.Asset{Key: domain.AssetKey{Market: domain.MarketB3, Symbol: "LTN-20290101"}, Class: domain.ClassGovBond, Active: true}
	f := newFixture(t, stock(petr), bond)
	src := &fakeQuotes{err: errors.New("rate limited")}

	r, err := f.svc.SyncQuotes(context.Background(), domain.MarketB3, src)
	if err == nil {
		t.Fatal("want the source error reported")
	}
	if len(src.asked) != 1 || src.asked[0] != petr {
		t.Errorf("asked %v, want only PETR4", src.asked)
	}
	if r.Rows != 1 {
		t.Errorf("rows = %d", r.Rows)
	}
	if q, err := f.feeds.LatestQuote(context.Background(), petr); err != nil || q.Price != 10 {
		t.Errorf("stored quote = %+v, %v", q, err)
	}
}

type fakeCompanyNews struct{ since map[domain.AssetKey]time.Time }

func (fakeCompanyNews) Name() string { return "fake-news" }
func (f fakeCompanyNews) CompanyNews(_ context.Context, k domain.AssetKey, since time.Time) ([]domain.NewsItem, error) {
	if k == gone {
		return nil, domain.ErrNotFound
	}
	f.since[k] = since
	return []domain.NewsItem{{URL: "https://x/" + string(k.Symbol), Title: "t", Source: "fake", PublishedAt: today, Assets: []domain.AssetKey{k}}}, nil
}

func TestSyncCompanyNews(t *testing.T) {
	f := newFixture(t, stock(aapl), stock(gone))
	src := fakeCompanyNews{since: map[domain.AssetKey]time.Time{}}

	r, err := f.svc.SyncCompanyNews(context.Background(), domain.MarketUS, src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Rows != 1 || r.Skipped != 1 {
		t.Errorf("result = %+v", r)
	}
	if got := src.since[aapl]; !got.Equal(today.Add(-72 * time.Hour)) {
		t.Errorf("since = %v, want 3 days back", got)
	}
	items, _ := f.feeds.NewsFor(context.Background(), aapl, d(2025, 1, 1), 10)
	if len(items) != 1 {
		t.Errorf("stored %d items", len(items))
	}
}

type fakeNews struct{ items []domain.NewsItem }

func (fakeNews) Name() string { return "fake-cvm" }
func (f fakeNews) News(_ context.Context, since time.Time) ([]domain.NewsItem, error) {
	return f.items, nil
}

func TestSyncNews(t *testing.T) {
	f := newFixture(t, stock(petr))
	item := domain.NewsItem{URL: "https://cvm/1", Title: "Fato Relevante", Source: "cvm:ipe", PublishedAt: today, Assets: []domain.AssetKey{petr}}
	for range 2 { // a repeat run must not duplicate
		if _, err := f.svc.SyncNews(context.Background(), fakeNews{items: []domain.NewsItem{item}}); err != nil {
			t.Fatal(err)
		}
	}
	items, _ := f.feeds.NewsFor(context.Background(), petr, d(2025, 1, 1), 10)
	if len(items) != 1 {
		t.Errorf("stored %d items, want 1", len(items))
	}
}

type fakeMacro struct{ since map[string]time.Time }

func (fakeMacro) Name() string     { return "fake-macro" }
func (fakeMacro) Serves() []string { return []string{domain.SeriesSelicTarget, domain.SeriesIPCA} }
func (f fakeMacro) Series(_ context.Context, code string, since time.Time) ([]domain.MacroPoint, error) {
	f.since[code] = since
	return []domain.MacroPoint{{Series: code, Date: d(2025, 3, 7), Value: 13.25, Source: "fake"}}, nil
}

func TestSyncMacroIsIncremental(t *testing.T) {
	f := newFixture(t)
	if err := f.feeds.UpsertMacro(context.Background(), []domain.MacroPoint{{Series: domain.SeriesIPCA, Date: d(2025, 2, 1), Value: 1.31}}); err != nil {
		t.Fatal(err)
	}
	src := fakeMacro{since: map[string]time.Time{}}
	r, err := f.svc.SyncMacro(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Assets != 2 || r.Rows != 2 {
		t.Errorf("result = %+v", r)
	}
	if got := src.since[domain.SeriesSelicTarget]; !got.Equal(d(2025, 1, 1)) {
		t.Errorf("first sync since = %v", got)
	}
	if got := src.since[domain.SeriesIPCA]; !got.Equal(d(2025, 2, 2)) {
		t.Errorf("repeat sync since = %v", got)
	}
}

type fakeBonds struct {
	since  time.Time
	quotes []domain.BondQuote
}

func (*fakeBonds) Name() string { return "fake-tesouro" }
func (f *fakeBonds) BondQuotes(_ context.Context, since time.Time) ([]domain.BondQuote, error) {
	f.since = since
	return f.quotes, nil
}

func TestSyncBondsRegistersBondAssets(t *testing.T) {
	f := newFixture(t, stock(petr))
	live := domain.AssetKey{Market: domain.MarketB3, Symbol: "NTNBP-20350515"}
	matured := domain.AssetKey{Market: domain.MarketB3, Symbol: "LTN-20250101"}
	src := &fakeBonds{quotes: []domain.BondQuote{
		{Asset: live, Name: "Tesouro IPCA+ 2035", Maturity: d(2035, 5, 15), Date: d(2025, 3, 7), BuyRate: 7.4},
		{Asset: live, Name: "Tesouro IPCA+ 2035", Maturity: d(2035, 5, 15), Date: d(2025, 3, 10), BuyRate: 7.45},
		{Asset: matured, Name: "Tesouro Prefixado 2025", Maturity: d(2025, 1, 1), Date: d(2024, 12, 30), BuyRate: 11},
	}}

	r, err := f.svc.SyncBonds(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Assets != 2 || r.Rows != 3 || !src.since.Equal(d(2025, 1, 1)) {
		t.Errorf("result = %+v, since %v", r, src.since)
	}
	if a, err := f.assets.GetAsset(context.Background(), live); err != nil || a.Class != domain.ClassGovBond || !a.Active || a.Name != "Tesouro IPCA+ 2035" {
		t.Errorf("live bond asset = %+v, %v", a, err)
	}
	if a, _ := f.assets.GetAsset(context.Background(), matured); a.Active {
		t.Error("matured bond should be inactive")
	}

	src.quotes = nil
	if _, err := f.svc.SyncBonds(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	if !src.since.Equal(d(2025, 3, 11)) {
		t.Errorf("repeat sync since = %v", src.since)
	}
}
