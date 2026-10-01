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

## Running locally

```bash
# backend
cd backend
go test ./...
go run ./cmd/shinrin api        # http://localhost:8080/healthz

# web
cd web
npm install
npm run dev                     # http://localhost:3000
```

Requires Go 1.24+ and Node 22+.
