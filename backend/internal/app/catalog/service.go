// Package catalog implements port.CatalogService on top of an AssetReader.
package catalog

import (
	"context"
	"fmt"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Service lists and looks up assets.
type Service struct {
	assets port.AssetReader
}

var _ port.CatalogService = (*Service)(nil)

// New builds the service around any asset store.
func New(assets port.AssetReader) *Service {
	return &Service{assets: assets}
}

// ListAssets returns the assets matching f.
func (s *Service) ListAssets(ctx context.Context, f port.AssetFilter) ([]domain.Asset, error) {
	assets, err := s.assets.ListAssets(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	return assets, nil
}

// GetAsset returns one asset or domain.ErrNotFound.
func (s *Service) GetAsset(ctx context.Context, key domain.AssetKey) (domain.Asset, error) {
	a, err := s.assets.GetAsset(ctx, key)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("get asset %s: %w", key, err)
	}
	return a, nil
}
