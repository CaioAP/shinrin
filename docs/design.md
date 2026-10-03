# Shinrin: architecture and design

Status: agreed design, version 1 · 2026-10-01 · Caio Alfonso with Claude
Companion: [data-sources.md](data-sources.md) (provider comparison, limits, links)

> **Not financial advice.** Shinrin is an educational and personal analysis tool. It never places orders, never connects to a brokerage, and every suggestion it shows is an opinion generated from public data, which can be wrong.

---

## 1. What Shinrin is

A web app that collects market data, filings and news for US and Brazilian (B3) investments, scores them the way an analyst would, and, if the user brings their own LLM API key, writes analyst-style reports suggesting where and when to invest according to the user's own risk profile.

**Hard rules**
1. Capture and process data only. No order execution, no brokerage or bank integration, no custody of user money.
2. Every core feature works without an LLM. AI adds narrative and judgment on top of numbers the system has already computed.
3. Every suggestion carries a visible disclaimer, its data sources and its "as of" date.
4. Free data only for now (official sources plus free API tiers).

## 2. Decisions log

These are the choices behind the design, kept for the blog post.

| # | Decision | Alternatives considered | Why |
|---|---|---|---|
| 1 | Nuxt frontend, Go backend | Full-stack Nuxt; Python backend | Caio's chosen stack. Go suits long-running scheduled jobs and a single deployable binary. |
| 2 | Cover US **and** B3 | US only (most free data) | Caio is in Brazil and wants to invest there. B3 has fewer free APIs, so we lean on official CVM and B3 files. |
| 3 | LLM is optional, bring your own key | Use the user's ChatGPT/Claude subscription | Consumer chat plans cannot be called by third-party apps. Only API keys work. |
| 4 | Free data only, for now | brapi Pro at about R$ 140/month | Keep costs at zero. The design lets brapi Pro slot in later as a source, mainly for B3 dividends and fundamentals. |
| 5 | Official sources as the backbone | Yahoo Finance scraping | B3, CVM, SEC, BCB, Tesouro and US Treasury data are free, unlimited in practice and stable. Yahoo has no official API and breaks without notice. |
| 6 | Multiple users with accounts | Single user, key in config | Portfolio visitors can sign up, and bring-your-own-key only makes sense per user. |
| 7 | Market data shared, fetched once | Fetch per user | Free-tier limits don't shrink as users join. |
| 8 | Universe: Ibovespa + S&P 500 + user additions | Every listed ticker | About 590 tickers fits free news and quote limits. Nightly bulk files still store everything. |
| 9 | Phased asset classes | Everything in v1 | v1 ships sooner. A generic asset model avoids redesign later. |
| 10 | Postgres + River job queue | Redis queues; cron + scripts; Temporal | One datastore for data and jobs, with retries and run history out of the box. |
| 11 | Two-layer analysis: deterministic scores, then LLM | LLM does everything | Numbers stay reproducible and testable. The LLM can't invent figures it wasn't given. |
| 12 | B3 dividends, JCP and splits from B3's own listed-company JSON service | brapi free tier (limited dividends); parsing CVM filings | Free, from the exchange, whole history in one call per company. It is undocumented, so the adapter parses strictly and fails loudly if the shape changes. |
| 13 | Adjust prices in-house from raw closes plus stored corporate actions | Store each vendor's adjusted close | A vendor's adjusted history is rewritten on every new dividend, so a stored copy goes stale. One method for both markets. |
| 14 | S&P 500 members from the community `datasets/s-and-p-500-companies` CSV | Scrape Wikipedia; licensed S&P data | Free, includes CIK and GICS sector. Not official, so membership is refreshed weekly and an empty list is refused. |
| 15 | Hand-written SQL on pgx with array (`unnest`) upserts; no sqlc yet | sqlc | About fifteen queries so far; one round trip writes thousands of rows. Revisit sqlc when the API read side grows. |
| 16 | One River queue per provider, one job at a time each | One shared queue | Rate limits are per provider, so a slow Tiingo backfill never blocks B3 or CVM. |
| 17 | Factor scores are percentiles against sector peers (market-wide when a sector has fewer than 5 members) | Absolute thresholds ("P/E under 10 is cheap") | What counts as cheap differs by sector and market; ranking within peers is how screens are built. Thresholds stay only where "good" has its own meaning (payout ratio, news tone). |
| 18 | Composite and views are recomputed on read; only factor scores are stored | Store one composite per profile | A profile change, or new macro data, needs no batch run, and the stored scores stay profile-free. |
| 19 | Banks, insurers and funds get their own metric sets (Strategy per asset kind) | One metric set for everything | EV/EBITDA and net debt describe neither a lender nor an FII. |
| 20 | AI reports are checked number by number against their input; a failing section is retried once, then dropped | Trust the model; reject the whole report | Keeps decision 11's promise (the LLM can't invent figures) without throwing away a mostly good note. |
| 21 | Anthropic through its official Go SDK; OpenAI-compatible APIs through plain HTTP | One generic client | The SDK gives structured output and server-side refusal fallback; the OpenAI format is a stable wire contract shared by many gateways. |

## 3. Scope by phase

| Phase | Contents |
|---|---|
| **v1** | Stocks (B3 + US index members), FIIs and REITs, dividends, government bonds (Tesouro Direto, US Treasuries), macro (Selic, CDI, IPCA, Fed rates, FX), news, scoring engine, watchlists, risk profile, optional AI reports |
| **v2** | Bank fixed-income calculator: CDB, RDB, LCI, LCA and US CDs. User enters an offer (e.g. 110% of CDI, 2 years), Shinrin projects the after-tax return and compares it with bonds and stocks. Also: Ollama support, brapi Pro adapter if budget allows |
| **Later** | ETFs, BDRs, crypto, backtesting of the scoring engine, email digests |

## 4. System overview

```
                         ┌──────────────────────────────────────────┐
  Browser ──HTTPS──▶     │  Nuxt app (SSR)                          │
                         │  dashboard · asset pages · watchlists    │
                         │  risk-profile onboarding · settings      │
                         └───────────────┬──────────────────────────┘
                                         │ REST/JSON (session cookie)
                         ┌───────────────▼──────────────────────────┐
                         │  Go binary  `shinrin api`                │
                         │  auth · users · watchlists · reads       │
                         │  enqueue AI report jobs                  │
                         └───────────────┬──────────────────────────┘
                                         │
                         ┌───────────────▼──────────────────────────┐
                         │  PostgreSQL                              │
                         │  market data · fundamentals · news       │
                         │  scores · reports · users · River jobs   │
                         └───────────────▲──────────────────────────┘
                                         │
                         ┌───────────────┴──────────────────────────┐
                         │  Go binary  `shinrin worker`             │
                         │  River scheduled + on-demand jobs        │
                         │  ingest ─▶ normalize ─▶ score ─▶ (LLM)   │
                         └──┬──────────┬──────────┬──────────┬──────┘
                            │          │          │          │
                     B3/CVM/BCB/   SEC/FRED/   Finnhub/   User's LLM provider
                     Tesouro       Treasury    brapi/RSS  (Anthropic, OpenAI-
                     (files)       (APIs)      (APIs)     compatible, Ollama)
```

One Go codebase, one binary, two roles (`shinrin api` and `shinrin worker`). Locally they can run in one process; in production they run as two processes so a long ingestion never slows the website.

## 5. Backend (Go)

Hexagonal (ports and adapters) layout. The full rules, and how each future package fits, are in `docs/conventions.md`.

```
cmd/shinrin/              composition root: `api`, `worker` (later `migrate`, `backfill`)
internal/
  domain/                 Asset, PriceBar, Quote, Fundamental, NewsItem, MacroPoint; later Score, Report
  port/                   driving ports (use cases) and driven ports (sources, repositories, LLM)
  app/                    application services, one package per area:
    system/ catalog/      (now)   ingest/ scoring/ profile/ report/ auth/  (later)
  adapter/in/             httpapi (REST); later River job handlers
  adapter/out/            memory (now); later postgres, b3cotahist, cvm, sec, bcb, tesouro,
                          treasury, fred, finnhub, brapi, rss, anthropic, openai, ollama, crypto
  config/                 env config
  archtest/               enforces the dependency rule
```

Pure calculations (indicators, price adjustment, factor scoring) live in `domain` subpackages so they stay free of I/O and easy to test.

Libraries (defaults, swappable, used only inside adapters): standard library `net/http` router (Go 1.22 patterns; `chi` if we outgrow it), `pgx` driver, `sqlc` for typed SQL, `River` for jobs, `goose` for migrations, `slog` logging.

**Source interface.** Each provider is a driven adapter implementing small ports from `internal/port/driven.go` (e.g. `PriceSource`, `FundamentalsSource`, `NewsSource`). Every row stored keeps `source` and `fetched_at`, so the UI can credit sources and a paid provider can replace a free one without touching the rest.

## 6. Data model (core tables)

```
assets            id, symbol, market (B3|US), class (stock|fii|reit|gov_bond|bank_fi|index),
                  name, sector, currency, isin, cvm_code / cik, is_index_member, active
prices_daily      asset_id, date, open, high, low, close, adj_close, volume, source
quotes_latest     asset_id, price, change_pct, as_of, source
corporate_actions asset_id, ex_date, type (dividend|jcp|split|bonus), value, source
fundamentals      asset_id, period_end, period_type (Q|FY), metric, value, source
                  (long format: revenue, net_income, equity, debt, fcf, shares, ...)
fii_reports       asset_id, month, distributed_income, nav, nav_per_share, vacancy, source
bond_quotes       asset_id, date, buy_rate, sell_rate, buy_price, sell_price, source
macro_series      code (selic, cdi, ipca, fed_funds, ust_10y, usdbrl...), date, value
news_items        id, published_at, source, url, title, summary, lang, hash
news_assets       news_id, asset_id, relevance
news_sentiment    news_id, asset_id, score (-1..1), method (lexicon|llm), model
scores            asset_id, as_of, factor (valuation|quality|growth|momentum|income|risk|sentiment),
                  value (0..100), details jsonb
users             id, email, password_hash, google_sub, created_at, locale, base_currency
risk_profiles     user_id, profile (conservative|moderate|aggressive), answers jsonb, horizon, updated_at
watchlists        id, user_id, name;  watchlist_items  watchlist_id, asset_id
llm_credentials   user_id, provider, model, enc_key, key_hint (last 4), monthly_cap, created_at
ai_reports        id, user_id, asset_id|null, kind (asset|portfolio|news_digest), as_of,
                  input_snapshot jsonb, output jsonb, model, tokens_in, tokens_out, cost_estimate
job_runs          (River tables) + a view for the status page
```

`prices_daily` is the only table that grows large (about 600 assets × 20+ years ≈ 3M rows, fine in plain Postgres). TimescaleDB stays an option if we ever store everything.

## 7. Routines (scheduler)

All times are cron jobs in River, stored in Postgres, retried with backoff, logged in `job_runs` and visible on a status page.

| Job | When | Source | Notes |
|---|---|---|---|
| `b3_prices_eod` | 20:30 BRT, weekdays | B3 COTAHIST daily file | All B3 tickers, then adjust for corporate actions |
| `us_prices_eod` | 18:30 ET, weekdays | Tiingo / FMP | Index members + watchlist tickers |
| `quotes_intraday` | Every 15 min in market hours | Finnhub (US), brapi free (B3, 30 min delay) | Watchlist and index members only |
| `cvm_filings` | Daily 07:00 BRT | CVM DFP/ITR/IPE, FII monthly reports | Incremental by file date |
| `sec_filings` | Daily 07:00 ET | SEC companyfacts | Only companies with new filings |
| `news_fetch` | Every 30 min, 07:00 to 22:00 | Finnhub news, CVM IPE, RSS | Dedupe by URL hash, tag tickers |
| `news_sentiment` | After `news_fetch` | Lexicon model (no LLM) | LLM sentiment only inside a user's AI report |
| `macro` | Daily | BCB SGS + Focus, FRED, Treasury | Selic, CDI, IPCA, PTAX, Fed funds, yield curve |
| `bonds` | Daily | Tesouro Transparente CSV, Treasury | Tesouro Direto titles and US yields |
| `index_membership` | Weekly | B3 and S&P constituent lists | Updates `is_index_member` |
| `scoring` | After EOD prices and filings | Internal | Recomputes factor scores |
| `ai_watchlist_reports` | User-scheduled (e.g. Monday 08:00) | User's LLM key | Respects the user's monthly cap |

Respecting rate limits: each source package has a token-bucket limiter set below the provider's free quota, and River's per-queue concurrency keeps one queue per provider.

Implemented in the data pipeline phase: `ibov_members`, `sp500_members` (the `index_membership` job), `b3_prices_eod`, `b3_corporate_actions`, `cvm_fundamentals`, `us_prices_eod` (Tiingo, with dividends and splits), `sec_fundamentals` and `indicators` (technicals and valuation ratios, ahead of `scoring`). Part 2 split the planned jobs per provider: `quotes_us` (Finnhub) and `quotes_b3` (brapi) for `quotes_intraday`; `news_us` (Finnhub company news) and `cvm_news` (CVM IPE) for `news_fetch`; `macro_br` (BCB SGS) and `macro_us` (FRED) for `macro`; `tesouro_bonds` for `bonds`. Intraday quotes run less often than every 15 minutes to stay inside the free quotas (README has the schedules). Tesouro titles are stored as `gov_bond` assets (symbol `CODE-YYYYMMDD`, inactive after maturity) so they can join watchlists later; price, quote, news and indicator routines skip them. Still to build: `news_sentiment`, RSS news, the Treasury auction feed, BCB Focus, FII monthly reports. Any routine can be run once by hand with `shinrin run <name>`.

## 8. Analysis engine (no LLM)

The engine follows how analysts actually screen investments: several independent factors, each scored relative to peers, then combined according to the investor's profile.

**Stocks (B3 and US)**
- **Valuation:** P/E, P/B (P/VP), EV/EBITDA, dividend yield, FCF yield, each compared with the sector median and the stock's own 5-year history.
- **Quality:** ROE, ROIC, margins and their stability, net debt/EBITDA, interest coverage.
- **Growth:** revenue and earnings growth (3y CAGR), last quarters' trend.
- **Momentum and technicals:** 3/6/12-month returns, price vs 50/200-day averages, RSI, MACD. These inform the "when" (timing), not the "what".
- **Income:** dividend history, payout ratio, consistency (JCP counted for B3).
- **Risk:** volatility, max drawdown, beta vs Ibovespa or S&P 500, leverage.
- **Sentiment:** recent news tone and material facts (fatos relevantes, 8-K).
- **Fair value estimates:** Graham number, Bazin (dividend-based, popular in Brazil) and a simple DCF using Selic or the US 10-year as the risk-free rate. Shown as ranges, never as a single target.

Banks and insurers get their own metric set, since EBITDA and debt ratios don't apply.

**FIIs and REITs:** dividend yield, P/VP, distribution consistency, vacancy, liquidity, segment (logistics, offices, paper/CRI), and for REITs FFO and AFFO.

**Government bonds:** real yield vs inflation expectations (Focus IPCA), spread vs Selic, duration and mark-to-market risk, after-tax return (IR regressivo for Tesouro).

**Risk profile and suggestions.** Onboarding asks a short questionnaire modeled on the Brazilian *suitability* process (horizon, loss tolerance, experience, income needs) and maps the user to conservative, moderate or aggressive. Each profile has allocation bands (e.g. conservative: 60–80% fixed income). The engine then:
1. Picks asset classes to fit the bands, using macro context (high Selic favors post-fixed bonds; inverted US curve signals caution).
2. Ranks assets within each class by the profile-weighted factor score.
3. Flags timing using momentum and valuation vs history ("expensive vs own history, wait" or "near 52-week low with improving quality").

Every score keeps a `details` breakdown, so the UI can always answer "why this rating?".

**As built (phase 3).** The rules are pure functions in `domain/scoring`; the `scoring` routine (weekdays 23:45 BRT, after `indicators`) stores factor scores, and `GET /api/v1/assets/{market}/{symbol}/analysis`, `/rankings` and `/outlook` derive the rest on read.

| Piece | Rule |
|---|---|
| Factor score | Weighted mean of metric percentiles within the peer group (same market, metric set and sector). Every score is 0-100 with higher better for the investor, so a high risk score means low risk. A factor with no data is absent, not zero. |
| Metric sets | Stocks: valuation (P/E, P/B, EV/EBITDA, FCF yield, dividend yield), quality (ROE, net and EBIT margin, net debt/EBITDA), growth (revenue and earnings growth 1y, revenue CAGR 3y), momentum (3/6/12-month return, price vs 200-day average), income (dividend yield, years paid out of 5, payout ratio), risk (volatility, max drawdown, leverage), sentiment (30-day headline tone). Financials drop EV and EBITDA metrics. FIIs and REITs: valuation, momentum, income, risk and sentiment only, until fund reports are ingested. |
| Profile weights | Conservative favours quality, income and risk (0.25/0.20/0.20); aggressive favours growth and momentum (0.25/0.20); moderate sits between. The composite renormalises over factors present and reports their coverage. |
| Fair value | Graham number (low end with a one-third margin of safety), Bazin (dividends / 8% to / 6%), and a five-year DCF on free cash flow at risk-free + equity premium (5% US, 6% B3), terminal growth 2% US / 4% B3, with a pessimistic and an optimistic case. The DCF is skipped with a note when no risk-free rate is stored (Selic for B3, the 10-year Treasury for the US). |
| Valuation view | Peer valuation score (65+ cheap, 35- expensive) plus where the price sits against the fair value ranges. |
| Timing view | Trend (price and 50-day vs 200-day average), MACD, RSI 70/30, 52-week position, and the valuation view, each a weighted signal: +1.5 or more is accumulate, -1.5 or less is avoid, otherwise wait. |
| Allocation bands | Conservative 60-80% fixed income, 10-30% stocks, 5-15% real estate; moderate 35-55 / 30-50 / 5-20; aggressive 10-30 / 50-75 / 5-20. A real Selic of 6% or more leans fixed income high and stocks and FIIs low; 3% or less does the opposite; an inverted US curve (10-year below 3-month) leans stocks low. |
| Sentiment | A small English and Portuguese finance lexicon over headline titles and summaries. Transparent and free; LLM sentiment only happens inside a user's AI report. |

## 9. AI layer (optional, bring your own key)

**Why bring-your-own-key.** Consumer plans (ChatGPT Plus, Claude Pro) only work inside their own apps. Third-party apps can only use API keys, billed per use to the key owner.

**Providers.** A Go interface `llm.Provider { Generate(ctx, Request) (Response, error) }` with adapters:
- Anthropic (Messages API)
- OpenAI-compatible (OpenAI, OpenRouter, and anything speaking that format)
- Ollama (v2, for self-hosted users)

Users pick the provider and model in settings. The app suggests a sensible default per provider and shows the model's price per million tokens.

**How a report is made.**
1. Build an *input snapshot* from the database: the asset's factor scores with details, key fundamentals, price stats, recent news headlines and material facts, macro context, and the user's risk profile. Everything in the snapshot is stored with the report.
2. Send it with a system prompt that tells the model to act as a sell-side style analyst, use only the numbers provided, cite which data point supports each claim, and present both bull and bear cases.
3. Require structured JSON output, validated in Go:

```json
{
  "summary": "...",
  "bull_case": ["..."], "bear_case": ["..."],
  "valuation_view": "cheap|fair|expensive",
  "timing_view": "accumulate|wait|avoid",
  "fit_for_profile": "good|partial|poor",
  "suggested_allocation_pct": {"min": 0, "max": 5},
  "key_risks": ["..."],
  "confidence": "low|medium|high",
  "cited_data": ["pe_ttm", "roe_5y", "news:123"]
}
```

4. Reject output that cites numbers not in the snapshot, retry once, then show the report without the failing section.
5. Show the disclaimer on every report, plus model, date and data "as of".

**Report types:** asset deep dive, watchlist/portfolio review (allocation vs profile bands), daily or weekly news digest for the user's watchlist.

**As built (phase 3).** Asset deep dives work end to end: `app/report` builds the snapshot (`domain/report.NewSnapshot`: scores, indicators, fair values with their assumptions, signals, the profile's band for the asset class, macro signals, up to 10 headlines from the last 30 days), calls the user's provider with the design's JSON schema as structured output, and validates the reply (`domain/report.Validate`): enums, allocation inside the band, citations that exist, and every figure in the prose matching a snapshot number within rounding (percentages, "bn"/"bilhões" magnitudes and Portuguese decimals understood). Failures go back to the model once with the list of problems; whatever still fails is dropped and named in `omitted`. Reports are stored in `ai_reports` with the snapshot, model and tokens. Until accounts exist (phase 4) reports run from `shinrin report`, with the operator's key in `SHINRIN_LLM_*`; per-user encrypted keys, the monthly cap and cost estimates come with accounts. Watchlist reviews and news digests also wait for accounts.

**Key security.**
- Keys are encrypted with AES-256-GCM using a per-row data key, which is wrapped by a master key held outside the database (env secret or cloud KMS).
- Keys are write-only from the browser: after saving, only the provider and last 4 characters are shown.
- Decrypted only in memory inside the worker, at call time. Never logged; log redaction middleware as a backstop.
- "Test key" button makes a tiny call to validate. Deleting the credential deletes the ciphertext.

**Cost control.** Users set a monthly cap. Before each run the app estimates tokens, shows the estimated cost, and refuses runs that would pass the cap. Each report stores tokens and estimated cost.

## 10. Users, auth and security

- Email + password (argon2id) and Google sign-in. Server-side sessions in Postgres, HttpOnly secure cookies, CSRF protection on state-changing requests.
- Per-user data: risk profile, watchlists, LLM credentials, AI reports. Everything else is shared market data.
- Rate limits per user on AI and expensive endpoints.
- LGPD basics: account deletion removes all personal data and keys; privacy page lists what is stored.

## 11. Frontend (Nuxt)

Pages for v1:
- **Landing** with the project story and prominent disclaimer (links to the blog post).
- **Onboarding:** sign up, risk questionnaire, optional LLM key.
- **Dashboard:** macro strip (Selic, CDI, IPCA, Fed funds, USD/BRL), watchlist with scores, latest news and material facts, suggested allocation vs profile bands.
- **Asset page:** price chart with indicators, factor scores with "why", fundamentals table, dividends, fair value ranges, news, AI report (generate or view).
- **Explore/screener:** filter and rank index members by factor, class and market.
- **Bonds:** Tesouro Direto titles and US yield curve, with after-tax comparisons.
- **Settings:** profile, LLM provider and key, monthly cap, scheduled AI reports.
- **Status:** routine run history and data freshness (also good portfolio material).

UI defaults: Nuxt UI or shadcn-vue for components, ECharts or Lightweight Charts for financial charts, i18n in Portuguese and English from day one, BRL and USD formatting.

The disclaimer appears in the footer of every page, at the top of every AI report and suggestion, and in onboarding as a required acknowledgement.

## 12. Deployment (default, open to change)

- Docker images for `shinrin api`, `shinrin worker` and the Nuxt app.
- Default: one small VPS with Docker Compose (Postgres, api, worker, Nuxt, Caddy for HTTPS). Lowest cost and full control.
- Alternative: Fly.io or Railway for the apps with a managed Postgres (Neon or Supabase) if you'd rather not manage a server.
- GitHub Actions: lint, test, build images, deploy on merge to main.
- Daily Postgres backups to object storage.

## 13. Regulatory note (Brazil)

In Brazil, publishing securities recommendations is regulated (CVM Resolução 20, securities analysts) and personalized investment advice is regulated too (CVM Resolução 19, consultants). This is an inference from the rules' scope, not legal advice. To stay on the safe side:
- Shinrin presents itself as an educational, personal analysis tool, not a recommendation service.
- Suggestions are only shown privately to the user who requested them, are generated from that user's own profile and own API key, and are never published or broadcast.
- The public demo shows scores and methodology, not "buy X" calls.
- Before promoting it widely or charging money, check with a lawyer familiar with CVM rules.

[Resolução CVM 20 (consolidated)](https://conteudo.cvm.gov.br/export/sites/cvm/legislacao/resolucoes/anexos/001/resol020consolid.pdf)

## 14. Mapping to the project threads

| Thread | Takes from this doc |
|---|---|
| 2. Data pipeline | Sections 5, 6, 7 and data-sources.md |
| 3. AI analysis engine | Sections 8 and 9 |
| 4. Web app and disclaimers | Sections 10, 11, 12, 13 |

## 15. Open items

- Repository: name and create it on GitHub (suggested `shinrin`, monorepo with `/backend` and `/web`).
- Hosting choice (VPS vs managed) and domain.
- Confirm exact Tiingo free limits. The adapter assumes about 50 requests an hour; a 500 distinct tickers per month cap would sit just under the S&P 500's 503 share classes.
- ~~Source for B3 stock dividends on the free plan~~: decided, B3's listed-company service (decision 12).
- Banks and insurers: CVM files them under a different chart of accounts, so their fundamentals are skipped until a financial-sector mapping exists.
