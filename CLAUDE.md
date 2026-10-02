# CLAUDE.md

Shinrin is a stock analysis web app for B3 and US markets: a Go backend
(`backend/`) and a Nuxt 4 app (`web/`). It only collects and analyses public
data; it never trades or connects to brokerages, and every screen shows a
"not financial advice" disclaimer.

**Read `docs/conventions.md` before changing code and follow it.** In short:

- Go backend is hexagonal: `domain` ← `port` ← `app` ← `adapter/{in,out}`,
  wired only in `cmd/shinrin/wire.go`. `internal/archtest` enforces the
  dependency rule; never weaken it to make a change pass.
- New data provider = new package in `backend/internal/adapter/out/` that
  implements source ports from `internal/port/driven.go`. New use case = new
  driving port + service in `internal/app/<area>/`.
- SOLID, small interfaces, constructor injection, design patterns only where
  they remove real coupling (see the table in conventions).
- Nuxt: file-based routing, auto-imports, composables wrap `useFetch`,
  browser calls only Nuxt `server/api/*` routes, which call Go through
  `server/utils/backend.ts`. Pinia only for client state.
- Keep `web/shared/types/api.ts` in sync with
  `backend/internal/adapter/in/httpapi/dto.go`.

Design and data sources: `docs/design.md`, `docs/data-sources.md`.

## Commands

```bash
# backend
cd backend && gofmt -l . && go vet ./... && go test ./...
go run ./cmd/shinrin api          # http://localhost:8080/healthz
go run ./cmd/shinrin migrate      # needs SHINRIN_DATABASE_URL (see backend/.env.example)
go run ./cmd/shinrin run <routine> # run one routine now; `worker` runs them on schedule
SHINRIN_TEST_DATABASE_URL=postgres://... go test ./...   # include Postgres/River tests

# web
cd web && npm ci && npm run lint && npm run typecheck
npm run dev                       # http://localhost:3000, needs the API running
```
