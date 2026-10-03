package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/httpapi"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/b3api"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/b3cotahist"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/bcb"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/brapi"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/cvm"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/finnhub"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/fred"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/postgres"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/sec"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/sp500"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/tesouro"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/tiingo"
	"github.com/CaioAP/shinrin/backend/internal/app/analysis"
	"github.com/CaioAP/shinrin/backend/internal/app/analytics"
	"github.com/CaioAP/shinrin/backend/internal/app/catalog"
	"github.com/CaioAP/shinrin/backend/internal/app/ingest"
	"github.com/CaioAP/shinrin/backend/internal/app/report"
	"github.com/CaioAP/shinrin/backend/internal/app/scoring"
	"github.com/CaioAP/shinrin/backend/internal/app/system"
	"github.com/CaioAP/shinrin/backend/internal/config"
	"github.com/CaioAP/shinrin/backend/internal/httpx"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// errNoDatabase is returned by roles that need Postgres when none is set.
var errNoDatabase = errors.New("SHINRIN_DATABASE_URL is required for this command")

// stores groups the repository ports. Postgres implements all of them; the
// memory adapters stand in when no database is configured.
type stores struct {
	assets       port.AssetRepository
	prices       port.PriceRepository
	fundamentals port.FundamentalRepository
	actions      port.CorporateActionRepository
	indicators   interface {
		port.IndicatorReader
		port.IndicatorWriter
	}
	quotes interface {
		port.QuoteReader
		port.QuoteWriter
	}
	news interface {
		port.NewsReader
		port.NewsWriter
	}
	macro    port.MacroRepository
	bonds    port.BondRepository
	analysis interface {
		port.ScoreReader
		port.ScoreWriter
		port.ReportWriter
	}
	health []port.HealthChecker
}

// container holds the wired application. Every role builds from it, so the
// api, the worker and one-off runs always share the same adapters.
type container struct {
	cfg       config.Config
	log       *slog.Logger
	pool      *pgxpool.Pool // nil without a database
	st        stores
	system    port.SystemService
	catalog   port.CatalogService
	ingest    *ingest.Service
	analytics *analytics.Service
	scoring   *scoring.Service
	analysis  port.AnalysisService
	report    port.ReportService
}

// build is the manual dependency injection for the whole app. Swap an adapter
// here (for example memory -> postgres) and nothing else changes.
func build(ctx context.Context, cfg config.Config, log *slog.Logger) (*container, func(), error) {
	c := &container{cfg: cfg, log: log}
	cleanup := func() {}

	if cfg.DatabaseURL != "" {
		pool, err := postgres.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		cleanup = pool.Close
		store := postgres.New(pool)
		c.pool = pool
		c.st = stores{
			assets: store, prices: store, fundamentals: store, actions: store, indicators: store,
			quotes: store, news: store, macro: store, bonds: store, analysis: store,
			health: []port.HealthChecker{store},
		}
	} else {
		data, feeds := memory.NewMarketDataStore(), memory.NewFeedStore()
		c.st = stores{
			assets: memory.NewAssetRepository(), prices: data, fundamentals: data, actions: data, indicators: data,
			quotes: feeds, news: feeds, macro: feeds, bonds: feeds, analysis: memory.NewAnalysisStore(),
		}
	}

	c.system = system.New(cfg.Version, c.st.health...)
	c.catalog = catalog.New(c.st.assets)
	c.ingest = ingest.New(ingest.Stores{
		Assets:       c.st.assets,
		Prices:       c.st.prices,
		Fundamentals: c.st.fundamentals,
		Actions:      c.st.actions,
		Quotes:       c.st.quotes,
		News:         c.st.news,
		Macro:        c.st.macro,
		Bonds:        c.st.bonds,
	}, ingest.Options{HistoryStart: cfg.HistoryStart, Logger: log})
	c.analytics = analytics.New(analytics.Stores{
		Assets:       c.st.assets,
		Prices:       c.st.prices,
		Fundamentals: c.st.fundamentals,
		Actions:      c.st.actions,
		Indicators:   c.st.indicators,
	}, nil, log)
	c.scoring = scoring.New(scoring.Stores{
		Assets:     c.st.assets,
		Indicators: c.st.indicators,
		Actions:    c.st.actions,
		News:       c.st.news,
		Scores:     c.st.analysis,
	}, nil, log)
	c.analysis = analysis.New(analysis.Stores{
		Assets:     c.st.assets,
		Scores:     c.st.analysis,
		Indicators: c.st.indicators,
		Macro:      c.st.macro,
	}, nil)
	c.report = report.New(report.Deps{
		Analysis: c.analysis,
		News:     c.st.news,
		Reports:  c.st.analysis,
		LLM:      llmConnector{},
	}, report.Options{Logger: log})
	return c, cleanup, nil
}

func (c *container) httpHandler() http.Handler {
	return httpapi.NewRouter(httpapi.Deps{
		System:   c.system,
		Catalog:  c.catalog,
		Analysis: c.analysis,
		Logger:   c.log,
	})
}

// Outbound clients, one per provider, sized under each free quota. Rates are
// documented in docs/data-sources.md.
const userAgent = "Shinrin/0.1 (+https://github.com/CaioAP/shinrin)"

func (c *container) b3Files() *b3cotahist.Source {
	return b3cotahist.New(httpx.NewClient(httpx.Options{UserAgent: userAgent, RequestsPerSecond: 1, Burst: 1, Retries: 3, Timeout: 10 * time.Minute}), "", nil)
}

func (c *container) b3API() *b3api.Client {
	return b3api.New(httpx.NewClient(httpx.Options{UserAgent: userAgent, RequestsPerSecond: 2, Burst: 1, Retries: 3, Timeout: time.Minute}), "")
}

func (c *container) cvm() *cvm.Client {
	return cvm.New(httpx.NewClient(httpx.Options{UserAgent: userAgent, RequestsPerSecond: 1, Burst: 1, Retries: 3, Timeout: 10 * time.Minute}), "", nil)
}

func (c *container) sp500() *sp500.Source {
	return sp500.New(httpx.NewClient(httpx.Options{UserAgent: userAgent, Retries: 3, Timeout: time.Minute}), "")
}

// sec returns nil when no contact User-Agent is configured: EDGAR blocks
// anonymous clients.
func (c *container) sec() *sec.Client {
	if c.cfg.SECUserAgent == "" {
		return nil
	}
	return sec.New(httpx.NewClient(httpx.Options{UserAgent: c.cfg.SECUserAgent, RequestsPerSecond: 8, Burst: 1, Retries: 3, Timeout: 2 * time.Minute}), "", "")
}

// tiingo returns nil without a token. The free tier allows about 50 requests
// an hour, so the first backfill of the S&P 500 takes a working day; later
// runs need one request per ticker.
func (c *container) tiingo() *tiingo.Client {
	if c.cfg.TiingoToken == "" {
		return nil
	}
	return tiingo.New(httpx.NewClient(httpx.Options{RequestsPerSecond: 45.0 / 3600, Burst: 1, Retries: 2, RetryBase: time.Minute, Timeout: 10 * time.Minute}), "", c.cfg.TiingoToken)
}

// finnhub returns nil without a token. The free tier allows 60 calls a
// minute; quotes and company news are one call per ticker.
func (c *container) finnhub() *finnhub.Client {
	if c.cfg.FinnhubToken == "" {
		return nil
	}
	return finnhub.New(httpx.NewClient(httpx.Options{UserAgent: userAgent, RequestsPerSecond: 50.0 / 60, Burst: 1, Retries: 2, RetryBase: 10 * time.Second, Timeout: time.Minute}), "", c.cfg.FinnhubToken)
}

// brapi returns nil without a token. The free plan allows one ticker per
// request and 15,000 requests a month, which the quotes_b3 schedule stays
// under for the Ibovespa.
func (c *container) brapi() *brapi.Client {
	if c.cfg.BrapiToken == "" {
		return nil
	}
	return brapi.New(httpx.NewClient(httpx.Options{UserAgent: userAgent, RequestsPerSecond: 2, Burst: 1, Retries: 2, Timeout: time.Minute}), "", c.cfg.BrapiToken, 1)
}

func (c *container) bcb() *bcb.Client {
	return bcb.New(httpx.NewClient(httpx.Options{UserAgent: userAgent, RequestsPerSecond: 1, Burst: 1, Retries: 3, Timeout: 2 * time.Minute}), "")
}

// fred returns nil without an API key.
func (c *container) fred() *fred.Client {
	if c.cfg.FREDAPIKey == "" {
		return nil
	}
	return fred.New(httpx.NewClient(httpx.Options{UserAgent: userAgent, RequestsPerSecond: 1, Burst: 1, Retries: 3, Timeout: 2 * time.Minute}), "", c.cfg.FREDAPIKey)
}

func (c *container) tesouro() *tesouro.Source {
	return tesouro.New(httpx.NewClient(httpx.Options{UserAgent: userAgent, Retries: 3, Timeout: 10 * time.Minute}), "")
}
