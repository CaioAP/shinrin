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
	// Assets, when set, restricts the ranking to these assets.
	Assets []domain.AssetKey
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
	// User owns the stored report; zero for the CLI.
	User    domain.UserID
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

// SessionToken is the secret a browser presents to stay signed in.
type SessionToken struct {
	Value     string
	ExpiresAt time.Time
}

// AccountService handles sign-up, sign-in, sessions and the risk profile.
type AccountService interface {
	// SignUp returns domain.ErrConflict when the email has an account.
	SignUp(ctx context.Context, email, password string) (domain.User, SessionToken, error)
	// SignIn returns domain.ErrUnauthorized for any wrong email or password.
	SignIn(ctx context.Context, email, password string) (domain.User, SessionToken, error)
	SignOut(ctx context.Context, token string) error
	// Authenticate returns domain.ErrUnauthorized for an unknown or expired
	// token.
	Authenticate(ctx context.Context, token string) (domain.User, error)
	SetRiskProfile(ctx context.Context, user domain.UserID, answers domain.SuitabilityAnswers) (domain.User, error)
	// DeleteAccount needs the password again; it returns
	// domain.ErrUnauthorized when it is wrong.
	DeleteAccount(ctx context.Context, user domain.UserID, password string) error
}

// WatchlistEntry is one asset of a watchlist with its scores, when scored.
type WatchlistEntry struct {
	Asset  domain.Asset
	Ranked *RankedAsset // nil when the asset has not been scored yet
}

// WatchlistService manages a user's watchlists.
type WatchlistService interface {
	List(ctx context.Context, user domain.UserID) ([]domain.Watchlist, error)
	Create(ctx context.Context, user domain.UserID, name string) (domain.Watchlist, error)
	Rename(ctx context.Context, user domain.UserID, id domain.WatchlistID, name string) error
	Delete(ctx context.Context, user domain.UserID, id domain.WatchlistID) error
	// AddAsset returns domain.ErrNotFound for an asset Shinrin does not track.
	AddAsset(ctx context.Context, user domain.UserID, id domain.WatchlistID, asset domain.AssetKey) error
	RemoveAsset(ctx context.Context, user domain.UserID, id domain.WatchlistID, asset domain.AssetKey) error
	// Entries returns the list's assets with scores for the profile, best
	// composite first, unscored assets last.
	Entries(ctx context.Context, user domain.UserID, id domain.WatchlistID, profile domain.RiskProfile) (domain.Watchlist, []WatchlistEntry, error)
}

// CredentialInput is what a user submits to save their LLM account. An
// empty APIKey keeps the key already saved (to change only the model or
// the cap).
type CredentialInput struct {
	Provider   string
	Model      string
	BaseURL    string
	APIKey     string
	MonthlyCap int
}

// CredentialService manages a user's own LLM account. The key is
// write-only: nothing here returns it.
type CredentialService interface {
	// Settings returns domain.ErrNotFound when the user saved no key.
	Settings(ctx context.Context, user domain.UserID) (domain.LLMSettings, error)
	Save(ctx context.Context, user domain.UserID, in CredentialInput) (domain.LLMSettings, error)
	// Test makes one tiny call with the saved key and returns
	// domain.ErrUpstream when the provider rejects it.
	Test(ctx context.Context, user domain.UserID) error
	Delete(ctx context.Context, user domain.UserID) error
}

// CredentialSource opens a user's saved key for one call. It is for other
// application services only and is never exposed over HTTP.
type CredentialSource interface {
	// CredentialFor returns domain.ErrNotFound when the user saved no key.
	CredentialFor(ctx context.Context, user domain.UserID) (LLMCredential, domain.LLMSettings, error)
	// Settings returns domain.ErrNotFound when the user saved no key.
	Settings(ctx context.Context, user domain.UserID) (domain.LLMSettings, error)
}

// UserReportService runs AI reports for signed-in users with their saved
// key and their monthly cap, and reads back their own reports.
type UserReportService interface {
	// Generate returns domain.ErrRateLimited at the monthly cap or while
	// another report of the user's is running.
	Generate(ctx context.Context, user domain.UserID, asset domain.AssetKey, profile domain.RiskProfile, lang string) (domain.Report, error)
	List(ctx context.Context, user domain.UserID, asset domain.AssetKey, limit int) ([]domain.Report, error)
	Get(ctx context.Context, user domain.UserID, id int64) (domain.Report, error)
	Usage(ctx context.Context, user domain.UserID) (domain.LLMUsage, error)
}
