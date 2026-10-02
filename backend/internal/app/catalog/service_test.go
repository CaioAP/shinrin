package catalog_test

import (
	"context"
	"errors"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/catalog"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

func seeded(t *testing.T) *catalog.Service {
	t.Helper()
	repo := memory.NewAssetRepository()
	err := repo.UpsertAssets(context.Background(), []domain.Asset{
		{Key: domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}, Class: domain.ClassStock, IndexMember: true, Active: true},
		{Key: domain.AssetKey{Market: domain.MarketB3, Symbol: "MXRF11"}, Class: domain.ClassFII, Active: true},
		{Key: domain.AssetKey{Market: domain.MarketUS, Symbol: "AAPL"}, Class: domain.ClassStock, IndexMember: true, Active: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return catalog.New(repo)
}

func TestListAssetsFilters(t *testing.T) {
	svc := seeded(t)
	yes := true
	got, err := svc.ListAssets(context.Background(), port.AssetFilter{Market: domain.MarketB3, IndexMember: &yes})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key.Symbol != "PETR4" {
		t.Fatalf("got %v, want only PETR4", got)
	}
}

func TestGetAssetNotFound(t *testing.T) {
	_, err := seeded(t).GetAsset(context.Background(), domain.AssetKey{Market: domain.MarketUS, Symbol: "NOPE"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
