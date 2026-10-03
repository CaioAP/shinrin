package port

import (
	"context"
	"time"

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

// RankFilter selects and orders a ranking. Zero values mean any market or
// class; Limit 0 means every asset.
type RankFilter struct {
	Market  domain.Market
	Class   domain.AssetClass
	Profile domain.RiskProfile
	Limit   int
}

// RankedAsset is one row of a ranking.
type RankedAsset struct {
	Asset     domain.Asset
	AsOf      time.Time
	Composite float64
	Coverage  float64
	Factors   []domain.FactorScore
}

// AnalysisService is the read side of the analysis engine: one asset's
// analysis, a ranked screener, and the allocation outlook for a profile.
type AnalysisService interface {
	// Analyze returns domain.ErrNotFound when the asset is unknown or has
	// not been scored yet.
	Analyze(ctx context.Context, asset domain.AssetKey, profile domain.RiskProfile) (domain.Analysis, error)
	Rank(ctx context.Context, f RankFilter) ([]RankedAsset, error)
	Outlook(ctx context.Context, profile domain.RiskProfile) (domain.Outlook, error)
}

// ReportRequest asks for an AI report on one asset.
type ReportRequest struct {
	Asset   domain.AssetKey
	Profile domain.RiskProfile
	// Lang is the report language: "en" (default) or "pt-BR".
	Lang       string
	Credential LLMCredential
}

// ReportService writes AI reports with the user's own LLM key.
type ReportService interface {
	Generate(ctx context.Context, req ReportRequest) (domain.Report, error)
}

// MarketService serves an asset's stored market data to the web app (price
// history, dividends, news) and the dashboard's macro strip.
type MarketService interface {
	// PriceHistory returns daily bars since from, oldest first, with
	// AdjClose recomputed for corporate actions. It returns
	// domain.ErrNotFound for an unknown asset.
	PriceHistory(ctx context.Context, asset domain.AssetKey, from time.Time) ([]domain.PriceBar, error)
	// Dividends returns cash distributions (dividends and JCP) since since,
	// newest first.
	Dividends(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.CorporateAction, error)
	// News returns up to limit of the asset's headlines from the last 90
	// days, newest first.
	News(ctx context.Context, asset domain.AssetKey, limit int) ([]domain.NewsItem, error)
	// Macro returns the headline macro numbers and which ones are missing.
	Macro(ctx context.Context) (domain.MacroStrip, error)
}
