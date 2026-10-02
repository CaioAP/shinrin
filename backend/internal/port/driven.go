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

// PriceReader reads stored daily bars.
type PriceReader interface {
	// PricesBetween returns bars with from <= date <= to, oldest first.
	PricesBetween(ctx context.Context, asset domain.AssetKey, from, to time.Time) ([]domain.PriceBar, error)
	// LatestPriceDate returns the date of the newest stored bar; ok is false
	// when the asset has no prices yet.
	LatestPriceDate(ctx context.Context, asset domain.AssetKey) (date time.Time, ok bool, err error)
}

// PriceWriter inserts or replaces daily bars by (asset, date).
type PriceWriter interface {
	UpsertPrices(ctx context.Context, bars []domain.PriceBar) error
}

// PriceRepository is the full price store.
type PriceRepository interface {
	PriceReader
	PriceWriter
}

// FundamentalReader reads stored fundamentals.
type FundamentalReader interface {
	FundamentalsSince(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.Fundamental, error)
	// LatestPeriodEnd returns the newest stored period end; ok is false when
	// the asset has no fundamentals yet.
	LatestPeriodEnd(ctx context.Context, asset domain.AssetKey) (end time.Time, ok bool, err error)
}

// FundamentalWriter inserts or replaces fundamentals by (asset, period end,
// period type, metric).
type FundamentalWriter interface {
	UpsertFundamentals(ctx context.Context, rows []domain.Fundamental) error
}

// FundamentalRepository is the full fundamentals store.
type FundamentalRepository interface {
	FundamentalReader
	FundamentalWriter
}

// CorporateActionReader reads stored corporate actions.
type CorporateActionReader interface {
	CorporateActionsSince(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.CorporateAction, error)
}

// CorporateActionWriter inserts or replaces corporate actions by (asset,
// ex-date, type).
type CorporateActionWriter interface {
	UpsertCorporateActions(ctx context.Context, actions []domain.CorporateAction) error
}

// CorporateActionRepository is the full corporate action store.
type CorporateActionRepository interface {
	CorporateActionReader
	CorporateActionWriter
}

// IndicatorReader reads computed indicators.
type IndicatorReader interface {
	// LatestIndicators returns domain.ErrNotFound when nothing was computed.
	LatestIndicators(ctx context.Context, asset domain.AssetKey) (domain.IndicatorSet, error)
}

// IndicatorWriter stores computed indicators, replacing any set with the same
// asset and date.
type IndicatorWriter interface {
	UpsertIndicators(ctx context.Context, sets []domain.IndicatorSet) error
}

// QuoteWriter replaces the latest quote per asset.
type QuoteWriter interface {
	UpsertQuotes(ctx context.Context, quotes []domain.Quote) error
}

// QuoteReader reads the latest quote.
type QuoteReader interface {
	// LatestQuote returns domain.ErrNotFound when no quote is stored.
	LatestQuote(ctx context.Context, asset domain.AssetKey) (domain.Quote, error)
}

// NewsWriter stores news items, deduplicated by URL, and links each to the
// tracked assets it mentions.
type NewsWriter interface {
	UpsertNews(ctx context.Context, items []domain.NewsItem) error
}

// NewsReader reads recent news.
type NewsReader interface {
	// NewsFor returns an asset's news published at or after since, newest
	// first.
	NewsFor(ctx context.Context, asset domain.AssetKey, since time.Time, limit int) ([]domain.NewsItem, error)
}

// MacroRepository stores macro series observations by (series, date).
type MacroRepository interface {
	UpsertMacro(ctx context.Context, points []domain.MacroPoint) error
	MacroSince(ctx context.Context, series string, since time.Time) ([]domain.MacroPoint, error)
	// LatestMacroDate returns ok false when the series is empty.
	LatestMacroDate(ctx context.Context, series string) (date time.Time, ok bool, err error)
}

// BondRepository stores government bond quotes by (asset, date).
type BondRepository interface {
	UpsertBondQuotes(ctx context.Context, quotes []domain.BondQuote) error
	// LatestBondDate returns ok false when no bond quote is stored.
	LatestBondDate(ctx context.Context) (date time.Time, ok bool, err error)
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

// MarketPriceSource fetches every bar a whole market published in a date
// range. Exchanges that publish bulk end-of-day files (B3 COTAHIST) implement
// it: one download covers every ticker, so fetching per asset would waste it.
type MarketPriceSource interface {
	Name() string
	Market() domain.Market
	// MarketPrices returns bars with from <= date <= to for every listed
	// ticker. Days without trading are simply absent.
	MarketPrices(ctx context.Context, from, to time.Time) ([]domain.PriceBar, error)
}

// UniverseSource lists an index's current members, which become the assets
// Shinrin tracks by default.
type UniverseSource interface {
	Name() string
	// Constituents returns the index members with Key, Class and Name set and
	// IndexMember true. Optional identifiers (ISIN, CIK, CNPJ) are filled when
	// the source knows them.
	Constituents(ctx context.Context, index domain.Index) ([]domain.Asset, error)
}

// QuoteSource fetches the latest (possibly delayed) quotes.
type QuoteSource interface {
	Name() string
	Quotes(ctx context.Context, assets []domain.AssetKey) ([]domain.Quote, error)
}

// FundamentalsSource fetches reported financial metrics, mapped to the
// domain.Metric* names. It returns domain.ErrNotFound for an asset it does not
// cover (an FII, a bank whose accounts it cannot map, an unknown ticker).
type FundamentalsSource interface {
	Name() string
	Fundamentals(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.Fundamental, error)
}

// CorporateActionSource fetches dividends, JCP, splits and bonuses. since is a
// lower bound the source may ignore (some return the full history in one
// call); storage is idempotent so extra rows are harmless.
type CorporateActionSource interface {
	Name() string
	CorporateActions(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.CorporateAction, error)
}

// NewsSource fetches market-wide news and filings published at or after
// since, each tagged with the assets it concerns (CVM material facts).
type NewsSource interface {
	Name() string
	News(ctx context.Context, since time.Time) ([]domain.NewsItem, error)
}

// CompanyNewsSource fetches one asset's news published at or after since
// (Finnhub company news). It returns domain.ErrNotFound for assets it does
// not cover.
type CompanyNewsSource interface {
	Name() string
	CompanyNews(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.NewsItem, error)
}

// MacroSource fetches observations of the domain.Series* codes it serves
// (BCB for Brazil, FRED for the US).
type MacroSource interface {
	Name() string
	// Serves lists the series codes this source provides.
	Serves() []string
	Series(ctx context.Context, series string, since time.Time) ([]domain.MacroPoint, error)
}

// BondSource fetches government bond quotes from since onwards, with the
// bond's identity (Asset, Name, Maturity) on every quote.
type BondSource interface {
	Name() string
	BondQuotes(ctx context.Context, since time.Time) ([]domain.BondQuote, error)
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
