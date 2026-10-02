package port

import (
	"context"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// --- Storage -----------------------------------------------------------------

// AssetReader reads the asset catalog.
type AssetReader interface {
	ListAssets(ctx context.Context, f AssetFilter) ([]domain.Asset, error)
	// GetAsset returns domain.ErrNotFound when the key is unknown.
	GetAsset(ctx context.Context, key domain.AssetKey) (domain.Asset, error)
}

// AssetWriter inserts or updates assets (used by ingestion routines).
type AssetWriter interface {
	UpsertAssets(ctx context.Context, assets []domain.Asset) error
}

// AssetRepository is the full catalog store. Depend on AssetReader or
// AssetWriter instead when only one side is needed.
type AssetRepository interface {
	AssetReader
	AssetWriter
}

// --- Market data providers ---------------------------------------------------
//
// One adapter per provider (B3 COTAHIST, CVM, SEC EDGAR, Finnhub, brapi, ...)
// implements whichever of these it can serve. Every value carries its Source
// so the UI can credit it and a paid provider can replace a free one.

// PriceSource fetches daily bars.
type PriceSource interface {
	Name() string
	DailyPrices(ctx context.Context, asset domain.AssetKey, from, to time.Time) ([]domain.PriceBar, error)
}

// QuoteSource fetches the latest (possibly delayed) quotes.
type QuoteSource interface {
	Name() string
	Quotes(ctx context.Context, assets []domain.AssetKey) ([]domain.Quote, error)
}

// FundamentalsSource fetches reported financial metrics.
type FundamentalsSource interface {
	Name() string
	Fundamentals(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.Fundamental, error)
}

// CorporateActionSource fetches dividends, JCP, splits and bonuses.
type CorporateActionSource interface {
	Name() string
	CorporateActions(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.CorporateAction, error)
}

// NewsSource fetches news and filings published after a point in time.
type NewsSource interface {
	Name() string
	News(ctx context.Context, since time.Time) ([]domain.NewsItem, error)
}

// MacroSource fetches macro series observations (selic, ipca, fed_funds, ...).
type MacroSource interface {
	Name() string
	Series(ctx context.Context, series string, since time.Time) ([]domain.MacroPoint, error)
}

// --- AI ---------------------------------------------------------------------

// LLMRequest is a provider-neutral prompt.
type LLMRequest struct {
	Model     string
	System    string
	Prompt    string
	MaxTokens int
	// JSONSchema, when set, asks the provider for structured output.
	JSONSchema []byte
}

// LLMResponse is a provider-neutral completion.
type LLMResponse struct {
	Text      string
	Model     string
	TokensIn  int
	TokensOut int
}

// LLMProvider is implemented once per vendor (Anthropic, OpenAI-compatible,
// Ollama). The user's own API key is bound when the adapter is constructed.
type LLMProvider interface {
	Name() string
	Generate(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

// --- Infrastructure ----------------------------------------------------------

// HealthChecker reports whether one dependency is reachable.
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) error
}

// Routine is a unit of scheduled work (ingest prices, score assets, ...). The
// scheduler adapter (River, in the data pipeline phase) decides when it runs.
type Routine interface {
	Name() string
	Run(ctx context.Context) error
}
