package port

import (
	"context"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// SystemInfo describes the running service to clients.
type SystemInfo struct {
	Name       string
	Version    string
	Disclaimer string
}

// HealthReport is the result of checking every dependency.
type HealthReport struct {
	OK     bool
	Checks map[string]string // dependency name -> "ok" or the error text
}

// SystemService answers questions about the service itself.
type SystemService interface {
	Info(ctx context.Context) SystemInfo
	Health(ctx context.Context) HealthReport
}

// AssetFilter narrows an asset listing. Zero values mean "any".
type AssetFilter struct {
	Market      domain.Market
	Class       domain.AssetClass
	IndexMember *bool
}

// CatalogService lists and looks up the assets Shinrin tracks.
type CatalogService interface {
	ListAssets(ctx context.Context, f AssetFilter) ([]domain.Asset, error)
	GetAsset(ctx context.Context, key domain.AssetKey) (domain.Asset, error)
}

// Routine is a unit of scheduled work (sync prices, compute indicators, ...).
// Application services expose their routines; the scheduler adapter (River)
// decides when each one runs. Run must be idempotent: a retried or repeated
// run must not duplicate data.
type Routine interface {
	Name() string
	Run(ctx context.Context) error
}
