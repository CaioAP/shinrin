package port

import (
	"context"
	"fmt"
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

// MacroReader reads macro series observations.
type MacroReader interface {
	// MacroSince returns observations dated at or after since, oldest first.
	MacroSince(ctx context.Context, series string, since time.Time) ([]domain.MacroPoint, error)
	// LatestMacroDate returns ok false when the series is empty.
	LatestMacroDate(ctx context.Context, series string) (date time.Time, ok bool, err error)
}

// MacroWriter stores macro series observations by (series, date).
type MacroWriter interface {
	UpsertMacro(ctx context.Context, points []domain.MacroPoint) error
}

// MacroRepository is the full macro store.
type MacroRepository interface {
	MacroReader
	MacroWriter
}

// BondRepository stores government bond quotes by (asset, date).
type BondRepository interface {
	UpsertBondQuotes(ctx context.Context, quotes []domain.BondQuote) error
	// LatestBondDate returns ok false when no bond quote is stored.
	LatestBondDate(ctx context.Context) (date time.Time, ok bool, err error)
}

// ScoreWriter stores factor scorecards, replacing any stored for the same
// asset and date.
type ScoreWriter interface {
	UpsertScorecards(ctx context.Context, cards []domain.Scorecard) error
}

// ScoreReader reads stored scorecards.
type ScoreReader interface {
	// LatestScorecard returns domain.ErrNotFound when the asset was never
	// scored.
	LatestScorecard(ctx context.Context, asset domain.AssetKey) (domain.Scorecard, error)
	// LatestScorecards returns each scored asset's newest scorecard in a
	// market.
	LatestScorecards(ctx context.Context, market domain.Market) ([]domain.Scorecard, error)
}

// ReportWriter stores generated AI reports.
type ReportWriter interface {
	// SaveReport stores r and returns its id.
	SaveReport(ctx context.Context, r domain.Report) (int64, error)
}

// --- Accounts ----------------------------------------------------------------

// UserRepository stores accounts.
type UserRepository interface {
	// CreateUser returns domain.ErrConflict when the email is taken.
	CreateUser(ctx context.Context, email, passwordHash string) (domain.User, error)
	// UserByEmail returns the user and their password hash, or
	// domain.ErrNotFound.
	UserByEmail(ctx context.Context, email string) (domain.User, string, error)
	// UserByID returns domain.ErrNotFound for an unknown id.
	UserByID(ctx context.Context, id domain.UserID) (domain.User, error)
	// PasswordHash returns the stored hash, or domain.ErrNotFound.
	PasswordHash(ctx context.Context, id domain.UserID) (string, error)
	SaveRiskProfile(ctx context.Context, id domain.UserID, p domain.RiskProfile, answers domain.SuitabilityAnswers, at time.Time) error
	// DeleteUser removes the account and everything it owns (sessions,
	// watchlists, credentials, reports).
	DeleteUser(ctx context.Context, id domain.UserID) error
}

// SessionRepository stores signed-in sessions by token hash.
type SessionRepository interface {
	CreateSession(ctx context.Context, s domain.Session) error
	// SessionByTokenHash returns domain.ErrNotFound for an unknown hash;
	// expiry is the caller's check.
	SessionByTokenHash(ctx context.Context, hash []byte) (domain.Session, error)
	DeleteSession(ctx context.Context, hash []byte) error
}

// WatchlistRepository stores watchlists. Every call is scoped to the owner,
// so one user can never read or change another's list: a list owned by
// someone else is domain.ErrNotFound.
type WatchlistRepository interface {
	ListWatchlists(ctx context.Context, user domain.UserID) ([]domain.Watchlist, error)
	GetWatchlist(ctx context.Context, user domain.UserID, id domain.WatchlistID) (domain.Watchlist, error)
	// CreateWatchlist returns domain.ErrConflict when the user already has
	// a list with that name.
	CreateWatchlist(ctx context.Context, user domain.UserID, name string) (domain.Watchlist, error)
	RenameWatchlist(ctx context.Context, user domain.UserID, id domain.WatchlistID, name string) error
	DeleteWatchlist(ctx context.Context, user domain.UserID, id domain.WatchlistID) error
	// AddWatchlistItem is idempotent.
	AddWatchlistItem(ctx context.Context, user domain.UserID, id domain.WatchlistID, asset domain.AssetKey) error
	RemoveWatchlistItem(ctx context.Context, user domain.UserID, id domain.WatchlistID, asset domain.AssetKey) error
}

// PasswordHasher hashes and checks passwords (argon2id in production).
type PasswordHasher interface {
	Hash(password string) (string, error)
	// Verify reports whether password matches hash.
	Verify(password, hash string) (bool, error)
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

// LLMCredential is a user's own provider account (bring your own key). The
// key is a secret: it is held in memory for one call and never logged or
// stored in plain text.
type LLMCredential struct {
	Provider string // anthropic | openai (any OpenAI-compatible API)
	Model    string // empty means the provider's default
	APIKey   string
	// BaseURL overrides the provider's endpoint (OpenRouter, a proxy).
	BaseURL string
}

// String hides the key, so a credential printed by mistake leaks nothing.
func (c LLMCredential) String() string {
	return fmt.Sprintf("{%s %s key:%s}", c.Provider, c.Model, KeyHint(c.APIKey))
}

// KeyHint is what may be shown of a key: its last four characters.
func KeyHint(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return "…" + key[len(key)-4:]
}

// LLMConnector builds a provider client for a credential (a factory, so
// each user's key gets its own client).
type LLMConnector interface {
	// Connect returns domain.ErrInvalid for an unknown provider.
	Connect(cred LLMCredential) (LLMProvider, error)
}

// --- Infrastructure ----------------------------------------------------------

// HealthChecker reports whether one dependency is reachable.
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) error
}
