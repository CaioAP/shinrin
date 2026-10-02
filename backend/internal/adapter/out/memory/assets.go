// Package memory provides in-memory driven adapters for tests and local runs
// before Postgres is wired in.
package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// AssetRepository is a concurrency-safe in-memory port.AssetRepository.
type AssetRepository struct {
	mu     sync.RWMutex
	assets map[domain.AssetKey]domain.Asset
}

var _ port.AssetRepository = (*AssetRepository)(nil)

// NewAssetRepository returns an empty repository.
func NewAssetRepository() *AssetRepository {
	return &AssetRepository{assets: make(map[domain.AssetKey]domain.Asset)}
}

// ListAssets returns matching assets sorted by market, then symbol.
func (r *AssetRepository) ListAssets(_ context.Context, f port.AssetFilter) ([]domain.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Asset, 0, len(r.assets))
	for _, a := range r.assets {
		if matches(a, f) {
			out = append(out, a)
		}
	}
	slices.SortFunc(out, func(a, b domain.Asset) int {
		return cmp.Or(cmp.Compare(a.Key.Market, b.Key.Market), cmp.Compare(a.Key.Symbol, b.Key.Symbol))
	})
	return out, nil
}

// GetAsset returns domain.ErrNotFound for unknown keys.
func (r *AssetRepository) GetAsset(_ context.Context, key domain.AssetKey) (domain.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.assets[key]
	if !ok {
		return domain.Asset{}, domain.ErrNotFound
	}
	return a, nil
}

// UpsertAssets inserts or replaces assets by key.
func (r *AssetRepository) UpsertAssets(_ context.Context, assets []domain.Asset) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, a := range assets {
		r.assets[a.Key] = a
	}
	return nil
}

func matches(a domain.Asset, f port.AssetFilter) bool {
	if f.Market != "" && a.Key.Market != f.Market {
		return false
	}
	if f.Class != "" && a.Class != f.Class {
		return false
	}
	if f.IndexMember != nil && a.IndexMember != *f.IndexMember {
		return false
	}
	return true
}
