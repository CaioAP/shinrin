# Shinrin

Shinrin collects market data, company filings and news for **Brazilian (B3)** and **US** investments, scores them the way an analyst would, and, if you bring your own LLM API key, writes analyst-style reports suggesting where and when to invest for your risk profile.

> **Not financial advice.** Shinrin is an educational and personal analysis tool built on public data, which can be delayed or wrong. It never places orders, never connects to a brokerage, and every suggestion is an opinion.

## Layout

| Path | What |
|---|---|
| `backend/` | Go service: `shinrin api` (HTTP API) and `shinrin worker` (scheduled routines) |
| `web/` | Nuxt app (dashboard, asset pages, onboarding) |
| `docs/design.md` | Architecture, data model, routines, scoring and AI design |
| `docs/data-sources.md` | Free data sources for B3 and US, with limits and terms |
| `docs/conventions.md` | Code conventions: hexagonal Go backend, SOLID, patterns, Nuxt conventions |

## Running locally

```bash
# backend
cd backend
go vet ./... && go test ./...
go run ./cmd/shinrin api        # http://localhost:8080/healthz

# web
cd web
npm ci
npm run lint && npm run typecheck
npm run dev                     # http://localhost:3000 (calls the API via NUXT_API_BASE)
```

Requires Go 1.26+ and Node 22+.

### Data pipeline

The worker needs Postgres. Settings are environment variables; see
`backend/.env.example`.

```bash
docker compose up -d postgres                      # or any Postgres 16
export SHINRIN_DATABASE_URL=postgres://shinrin:shinrin@localhost:5432/shinrin?sslmode=disable
cd backend
go run ./cmd/shinrin migrate                       # Shinrin tables + River job tables

# First load, once, in this order (the universe decides which tickers are synced)
go run ./cmd/shinrin run ibov_members sp500_members
go run ./cmd/shinrin run b3_prices_eod b3_corporate_actions cvm_fundamentals
go run ./cmd/shinrin run us_prices_eod sec_fundamentals   # need SHINRIN_TIINGO_TOKEN / SHINRIN_SEC_USER_AGENT
go run ./cmd/shinrin run indicators
go run ./cmd/shinrin run macro_br tesouro_bonds cvm_news
go run ./cmd/shinrin run quotes_us news_us quotes_b3 macro_us   # need Finnhub / brapi / FRED keys

go run ./cmd/shinrin worker                        # then keep everything fresh on schedule
```

| Routine | Source | Schedule |
|---|---|---|
| `ibov_members`, `sp500_members` | B3 index portfolio, S&P 500 list | Mondays 06:00 |
| `b3_prices_eod` | B3 COTAHIST files | weekdays 20:30 BRT |
| `b3_corporate_actions` | B3 listed-company service | weekdays 21:00 BRT |
| `cvm_fundamentals` | CVM DFP and ITR | daily 07:00 BRT |
| `us_prices_eod` | Tiingo (prices, dividends, splits) | weekdays 18:30 ET |
| `sec_fundamentals` | SEC EDGAR companyfacts | daily 07:00 ET |
| `indicators` | computed from the above | weekdays 23:30 BRT |
| `quotes_us` | Finnhub | every 30 min, 09:00 to 16:30 ET, weekdays |
| `news_us` | Finnhub company news | 07:15, 12:15, 18:15 ET |
| `quotes_b3` | brapi (one ticker per call on the free plan) | 10, 12, 14, 16, 18h BRT, weekdays |
| `cvm_news` | CVM IPE (material facts, market announcements) | every 30 min, 07:00 to 23:30 BRT |
| `macro_br` | BCB SGS (Selic, CDI, IPCA, PTAX) | 09:00 and 19:00 BRT |
| `macro_us` | FRED (Fed funds, Treasury 3M/2Y/10Y, CPI) | 09:00 and 18:00 ET |
| `tesouro_bonds` | Tesouro Transparente | weekdays 10:00 and 19:00 BRT |

Routines whose key is not set are left out of the worker with a warning.
