package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func TestFeeds(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	bond := domain.AssetKey{Market: domain.MarketB3, Symbol: "NTNBP-20350515"}
	if err := s.UpsertAssets(ctx, []domain.Asset{
		{Key: petr, Class: domain.ClassStock, Active: true},
		{Key: aapl, Class: domain.ClassStock, Active: true},
		{Key: bond, Class: domain.ClassGovBond, Name: "Tesouro IPCA+ 2035", Active: true},
	}); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2025, 3, 10, 15, 0, 0, 0, time.UTC)

	// Quotes: newest wins, an older one does not overwrite it.
	if err := s.UpsertQuotes(ctx, []domain.Quote{{Asset: aapl, Price: 227, ChangePct: 0.5, AsOf: t0, Source: "finnhub"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertQuotes(ctx, []domain.Quote{{Asset: aapl, Price: 200, AsOf: t0.Add(-time.Hour), Source: "finnhub"}}); err != nil {
		t.Fatal(err)
	}
	if q, err := s.LatestQuote(ctx, aapl); err != nil || q.Price != 227 || !q.AsOf.Equal(t0) {
		t.Errorf("LatestQuote = %+v, %v", q, err)
	}
	if _, err := s.LatestQuote(ctx, petr); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("no quote err = %v", err)
	}

	// News: deduplicated by URL, linked to known assets only.
	item := domain.NewsItem{URL: "https://example.com/a", Title: "Fato Relevante: dividendos", Lang: "pt", Source: "cvm:ipe", PublishedAt: t0,
		Assets: []domain.AssetKey{petr, {Market: domain.MarketB3, Symbol: "PETR3"}}}
	for range 2 {
		if err := s.UpsertNews(ctx, []domain.NewsItem{item, item}); err != nil {
			t.Fatal(err)
		}
	}
	news, err := s.NewsFor(ctx, petr, t0.Add(-24*time.Hour), 10)
	if err != nil || len(news) != 1 || news[0].Title != item.Title || !news[0].PublishedAt.Equal(t0) {
		t.Errorf("NewsFor = %+v, %v", news, err)
	}

	// Macro.
	if err := s.UpsertMacro(ctx, []domain.MacroPoint{
		{Series: "selic", Date: d(2025, 3, 7), Value: 13.25, Source: "bcb_sgs"},
		{Series: "selic", Date: d(2025, 3, 10), Value: 13.25, Source: "bcb_sgs"},
	}); err != nil {
		t.Fatal(err)
	}
	if last, ok, err := s.LatestMacroDate(ctx, "selic"); err != nil || !ok || !last.Equal(d(2025, 3, 10)) {
		t.Errorf("LatestMacroDate = %v %v %v", last, ok, err)
	}
	if pts, err := s.MacroSince(ctx, "selic", d(2025, 3, 8)); err != nil || len(pts) != 1 {
		t.Errorf("MacroSince = %+v, %v", pts, err)
	}
	if _, ok, _ := s.LatestMacroDate(ctx, "ipca"); ok {
		t.Error("empty series should not be ok")
	}

	// Bonds.
	if err := s.UpsertBondQuotes(ctx, []domain.BondQuote{{Asset: bond, Maturity: d(2035, 5, 15), Date: d(2025, 3, 10), BuyRate: 7.45, SellRate: 7.57, BuyPrice: 1987.26, SellPrice: 1965.01, Source: "tesouro"}}); err != nil {
		t.Fatal(err)
	}
	if last, ok, err := s.LatestBondDate(ctx); err != nil || !ok || !last.Equal(d(2025, 3, 10)) {
		t.Errorf("LatestBondDate = %v %v %v", last, ok, err)
	}
}
