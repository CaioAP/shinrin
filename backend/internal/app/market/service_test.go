package market_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/market"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

var (
	ctx   = context.Background()
	itub  = domain.AssetKey{Market: domain.MarketB3, Symbol: "ITUB4"}
	clock = time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
)

func day(n int) time.Time { return clock.AddDate(0, 0, n) }

func newService(t *testing.T) *market.Service {
	t.Helper()
	assets, data, feeds := memory.NewAssetRepository(), memory.NewMarketDataStore(), memory.NewFeedStore()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(assets.UpsertAssets(ctx, []domain.Asset{{Key: itub, Class: domain.ClassStock, Active: true}}))
	must(data.UpsertPrices(ctx, []domain.PriceBar{
		{Asset: itub, Date: day(-3), Close: 40}, {Asset: itub, Date: day(-2), Close: 40}, {Asset: itub, Date: day(-1), Close: 39},
	}))
	must(data.UpsertCorporateActions(ctx, []domain.CorporateAction{
		{Asset: itub, ExDate: day(-1), Type: domain.ActionDividend, Value: 1},
		{Asset: itub, ExDate: day(-400), Type: domain.ActionJCP, Value: 0.2},
		{Asset: itub, ExDate: day(-30), Type: domain.ActionSplit, Value: 2},
	}))
	must(feeds.UpsertNews(ctx, []domain.NewsItem{
		{URL: "a", Title: "new", PublishedAt: day(-1), Assets: []domain.AssetKey{itub}},
		{URL: "b", Title: "old", PublishedAt: day(-200), Assets: []domain.AssetKey{itub}},
	}))
	return market.New(market.Stores{Assets: assets, Prices: data, Actions: data, News: feeds, Macro: feeds}, func() time.Time { return clock })
}

func TestPriceHistoryAdjustsForDividends(t *testing.T) {
	bars, err := newService(t).PriceHistory(ctx, itub, day(-10))
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 3 || bars[2].AdjClose != 39 || bars[0].AdjClose != 39 {
		t.Fatalf("bars = %+v, want the pre-dividend closes scaled to 39", bars)
	}
}

func TestUnknownAssetIsNotFound(t *testing.T) {
	_, err := newService(t).PriceHistory(ctx, domain.AssetKey{Market: domain.MarketB3, Symbol: "XXXX3"}, day(-10))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDividendsAreCashOnlyNewestFirst(t *testing.T) {
	got, err := newService(t).Dividends(ctx, itub, day(-500))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Type != domain.ActionDividend || got[1].Type != domain.ActionJCP {
		t.Fatalf("dividends = %+v", got)
	}
}

func TestNewsLooksBackNinetyDays(t *testing.T) {
	got, err := newService(t).News(ctx, itub, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "new" {
		t.Fatalf("news = %+v", got)
	}
}

func TestMacroReportsMissingSeries(t *testing.T) {
	s, err := newService(t).Macro(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Items) != 0 || len(s.Missing) != 7 {
		t.Fatalf("strip = %+v, want everything missing", s)
	}
}
