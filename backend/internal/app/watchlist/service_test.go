package watchlist_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/analysis"
	"github.com/CaioAP/shinrin/backend/internal/app/watchlist"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

var (
	ctx  = context.Background()
	petr = domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}
	aapl = domain.AssetKey{Market: domain.MarketUS, Symbol: "AAPL"}
)

func setup(t *testing.T) (*watchlist.Service, domain.UserID, domain.UserID) {
	t.Helper()
	assets, scores, feeds, data, accts := memory.NewAssetRepository(), memory.NewAnalysisStore(), memory.NewFeedStore(), memory.NewMarketDataStore(), memory.NewAccountStore()
	_ = assets.UpsertAssets(ctx, []domain.Asset{
		{Key: petr, Class: domain.ClassStock, Active: true},
		{Key: aapl, Class: domain.ClassStock, Active: true},
	})
	_ = scores.UpsertScorecards(ctx, []domain.Scorecard{{Asset: petr, AsOf: time.Now(), Factors: []domain.FactorScore{{Factor: domain.FactorValuation, Value: 80}}}})
	an := analysis.New(analysis.Stores{Assets: assets, Scores: scores, Indicators: data, Macro: feeds}, nil)
	a, _ := accts.CreateUser(ctx, "a@example.com", "h")
	b, _ := accts.CreateUser(ctx, "b@example.com", "h")
	return watchlist.New(watchlist.Deps{Lists: accts, Assets: assets, Analysis: an}), a.ID, b.ID
}

func TestWatchlistLifecycle(t *testing.T) {
	svc, alice, bob := setup(t)
	w, err := svc.Create(ctx, alice, "  Dividends ")
	if err != nil || w.Name != "Dividends" {
		t.Fatalf("Create = %+v, %v", w, err)
	}
	if _, err := svc.Create(ctx, alice, "Dividends"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate name err = %v", err)
	}
	if _, err := svc.Create(ctx, alice, " "); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("blank name err = %v", err)
	}
	for _, k := range []domain.AssetKey{aapl, petr, petr} {
		if err := svc.AddAsset(ctx, alice, w.ID, k); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.AddAsset(ctx, alice, w.ID, domain.AssetKey{Market: domain.MarketUS, Symbol: "NOPE"}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown asset err = %v", err)
	}

	_, entries, err := svc.Entries(ctx, alice, w.ID, domain.ProfileModerate)
	if err != nil || len(entries) != 2 {
		t.Fatalf("Entries = %+v, %v", entries, err)
	}
	if entries[0].Asset.Key != petr || entries[0].Ranked == nil || entries[1].Asset.Key != aapl || entries[1].Ranked != nil {
		t.Errorf("want scored PETR4 first, unscored AAPL last: %+v", entries)
	}

	// Another user can neither see nor change Alice's list.
	if _, _, err := svc.Entries(ctx, bob, w.ID, domain.ProfileModerate); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("bob read alice's list: %v", err)
	}
	if err := svc.AddAsset(ctx, bob, w.ID, petr); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("bob wrote alice's list: %v", err)
	}
	if err := svc.Delete(ctx, bob, w.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("bob deleted alice's list: %v", err)
	}

	if err := svc.RemoveAsset(ctx, alice, w.ID, aapl); err != nil {
		t.Fatal(err)
	}
	if err := svc.Rename(ctx, alice, w.ID, "Income"); err != nil {
		t.Fatal(err)
	}
	ls, _ := svc.List(ctx, alice)
	if len(ls) != 1 || ls[0].Name != "Income" || len(ls[0].Assets) != 1 {
		t.Errorf("List = %+v", ls)
	}
	if err := svc.Delete(ctx, alice, w.ID); err != nil {
		t.Fatal(err)
	}
	if ls, _ := svc.List(ctx, alice); len(ls) != 0 {
		t.Errorf("list survived delete: %+v", ls)
	}
}
