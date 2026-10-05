# Shinrin code conventions

These rules apply to every change, human or AI. They exist so the data
pipeline, the analysis engine and the web app can grow without turning into a
tangle. When a rule gets in the way, change the rule here first, in the same PR.

The four principles:

1. **Ports and adapters (hexagonal architecture)** in the Go backend.
2. **SOLID** everywhere.
3. **Design patterns where they remove real duplication or coupling**, never for decoration.
4. **The Nuxt way** on the frontend: use the framework's conventions before inventing our own.

---

## Backend (Go)

### Layout

```
backend/
  cmd/shinrin/          composition root: config, wiring (wire.go), routine registry (worker.go),
                        and the `api`, `worker`, `migrate` and `run` commands
  internal/
    domain/             entities, value objects, domain errors. Standard library only.
    port/               interfaces: driving.go (use cases), driven.go (what the app needs)
    app/<area>/         application services, one package per use-case area
    adapter/in/<name>/  driving adapters: httpapi (REST), jobs (River scheduler)
    adapter/out/<name>/ driven adapters: memory, postgres, b3cotahist, b3api, cvm, sec, tiingo,
                        sp500, finnhub, brapi, bcb, fred, tesouro; anthropic and openai (LLMs)
    httpx/              outbound HTTP decorators (user agent, rate limit, retry) for providers
    config/             env config, read once in cmd/
    archtest/           tests that enforce the dependency rule below
```

### The dependency rule

Dependencies point inward. `internal/archtest` fails the build if any of these
are broken (test files are exempt, so a service test may use the memory
adapter):

| Layer | May import (inside `internal/`) |
|---|---|
| `domain` | other `domain` packages only (e.g. `domain/indicators` uses `domain`) |
| `port` | `domain` |
| `app/*` | `domain`, `port` |
| `adapter/in/*`, `adapter/out/*` | `domain`, `port` |
| `httpx` | nothing (it decorates `net/http`; `cmd` hands the result to adapters) |
| `config` | nothing |
| `cmd/shinrin` | everything (it is the only place that does) |

Consequences:
- An adapter never imports another adapter or an application service. The
  HTTP adapter calls a driving port; it does not know which service is behind it.
- Third-party libraries (pgx, River, an LLM SDK, chi) appear only in adapters
  and `cmd/`. `domain` uses the standard library only.
- New top-level packages under `internal/` must be added to
  `archtest.allowed` and to the table above.

### Ports

- **Driving ports** (`port/driving.go`) are the use cases: `SystemService`,
  `CatalogService`, and `Routine` (scheduled work: the `ingest` and
  `analytics` services expose their syncs as routines, which the `jobs`
  adapter runs on a cron), `AnalysisService` (analysis, rankings, outlook),
  `MarketService` (price history, dividends, news and the macro strip for
  the web app), `AccountService` (sign-up, sessions, risk profile),
  `WatchlistService` and `ReportService` (AI reports).
- **Driven ports** (`port/driven.go`) are the outside world: repositories
  (`AssetReader`/`AssetWriter`, `PriceReader`/`PriceWriter`,
  `FundamentalReader`/`FundamentalWriter`, `CorporateActionReader`/`Writer`,
  `IndicatorReader`/`Writer`, `QuoteReader`/`Writer`, `NewsReader`/`Writer`,
  `MacroRepository`, `BondRepository`), market data (`UniverseSource`,
  `MarketPriceSource`, `PriceSource`, `QuoteSource`, `FundamentalsSource`,
  `CorporateActionSource`, `NewsSource`, `CompanyNewsSource`, `MacroSource`,
  `BondSource`), analysis (`ScoreReader`/`ScoreWriter`, `ReportWriter`), AI
  (`LLMProvider`, and `LLMConnector`, the factory that binds a user's key),
  infrastructure (`HealthChecker`).
- A source port returns `domain.ErrNotFound` for an asset it does not cover;
  ingestion skips those quietly and counts every other error as a failure.
- Ports mention only domain types and the standard library.
- Keep interfaces small. Split read and write sides (`AssetReader` /
  `AssetWriter`) and let consumers depend on the side they use.
- Every implementation asserts its port at compile time:
  `var _ port.AssetRepository = (*AssetRepository)(nil)`.

### Application services

- One package per area under `internal/app/`, exposing `New(...)` that takes
  its driven ports as interfaces and returns `*Service`.
- Business rules live in `domain` (pure functions and methods) or in services.
  Never in handlers, never in repositories.
- Wrap errors with context (`fmt.Errorf("get asset %s: %w", key, err)`) and
  keep domain sentinels (`domain.ErrNotFound`, `domain.ErrInvalid`) testable
  with `errors.Is`.

### Adapters

- **Driving (`adapter/in`)**: parse input, call a port, render output.
  `httpapi` converts domain values to DTOs (`dto.go`) so the JSON contract does
  not change when a domain field is renamed, and maps domain errors to status
  codes in one place (`respond.go`). Middleware is a decorator chain
  (`middleware.go`).
- **Driven (`adapter/out`)**: one package per technology or provider. A data
  provider package implements every source port it can serve and nothing else
  (for example `b3cotahist` implements `MarketPriceSource`; `tiingo`
  implements `PriceSource` and `CorporateActionSource`). Parsing and any
  provider-specific caching belong inside the adapter. Generic politeness
  (rate limit, retry, User-Agent) comes from the `*http.Client` that
  `cmd/shinrin/wire.go` builds with `httpx` and injects, so adapter tests use
  `httptest` servers and real fixtures in the provider's own format.
- Data adapters are tested against fixtures in the provider's real format
  (zip, fixed-width text, CSV, JSON) served by `httptest`. Postgres and River
  tests run when `SHINRIN_TEST_DATABASE_URL` is set (CI sets it) and skip
  otherwise.
- Every stored value keeps its `Source`.

### Wiring (dependency injection)

- Manual constructor injection in `cmd/shinrin/wire.go`. No DI framework, no
  globals, no `init()` side effects.
- `config.Load` is the only reader of environment variables. Values are passed
  down as plain structs.
- Swapping an implementation (memory to Postgres, free to paid provider) is a
  one-line change in `wire.go`.

### Design patterns in use (and where the next ones go)

| Pattern | Where | Why |
|---|---|---|
| Adapter | every `adapter/out` package | wraps a provider's API behind a port |
| Strategy | source ports passed per routine (`ingest.PricesRoutine(name, market, source, ...)`), `LLMProvider`, scoring models (`scoring.ModelFor`: stock, financials, fund) | the same ingestion code serves B3 and US from different providers, chosen at wiring time; banks and FIIs are scored with their own metric sets |
| Factory | `port.LLMConnector` (registry in `cmd/shinrin/llm.go`) | each user's key gets its own provider client; a new vendor is one registry entry |
| Decorator | HTTP middleware; `httpx` round trippers (user agent, rate limit, retry) | add behaviour without touching the wrapped type |
| Composite | `system.Service` aggregates `HealthChecker`s | one health answer from many dependencies |
| Repository | `AssetReader` / `AssetWriter` | storage behind an interface |
| Registry | `routines()` in `cmd/shinrin/worker.go`; one River worker dispatching by routine name | the worker discovers routines from one list |

Prefer a decorator over a flag (`NewCachedPriceSource(inner)`, not
`PriceSource{cache: true}`), and a new strategy over a `switch` on provider
name in business code.

### Go style

- `gofmt`, `go vet` and `go test ./...` must pass. Tests are table-driven where
  there is more than one case, live next to the code, and use the `_test`
  package to test through the public API.
- Test a service with fakes or the memory adapter; test an adapter with fakes
  of the port it calls (see `adapter/in/httpapi/router_test.go`).
- Exported identifiers have doc comments. Comments say why, not what.
- `context.Context` is the first argument of anything that does I/O.
- Logging is `log/slog`, injected, structured. Never log secrets or API keys.
- Analysis numbers are `float64` (Shinrin analyses money, it never moves it).
- Analysis rules (scores, fair values, views, report checks) are pure
  functions in `domain/scoring` and `domain/report`; services only load
  inputs and store results. An LLM never supplies a number: it explains the
  snapshot it is given, and `domain/report.Validate` enforces that.
- LLM API keys are secrets: never log them, never put them in prompts or
  errors, and print credentials only through `port.LLMCredential.String`.

---

## Frontend (Nuxt 4)

Follow the Nuxt directory conventions; nothing is registered by hand.

```
web/
  app/
    app.vue              root: <NuxtLayout><NuxtPage/></NuxtLayout>
    error.vue            error page (404 and others)
    layouts/default.vue  header, <slot/>, DisclaimerFooter
    pages/               file-based routing (pages/assets/[market]/[symbol].vue)
    components/          auto-imported components, PascalCase
    composables/         useXxx() data access and shared logic, auto-imported
    stores/              Pinia setup stores, auto-imported (usePreferencesStore)
    utils/               pure helpers (formatting), auto-imported
  server/
    api/                 Nitro routes the browser calls (BFF)
    utils/backend.ts     backendFetch(): the only caller of the Go API
  shared/types/          API types, auto-imported in app/ and server/
```

### Data flow

```
page/component ─▶ composable (useAssets) ─▶ useFetch('/api/assets')
  ─▶ server/api/assets/index.get.ts ─▶ backendFetch() ─▶ Go /api/v1/assets
```

- **The browser only calls Nuxt's own `/api/*` routes.** The Go API URL is a
  private runtime config value (`NUXT_API_BASE`). This keeps cookies same-origin,
  hides the backend, and gives one place for auth headers later.
- **Server routes are thin.** They forward allowed params to the Go API via
  `backendFetch` and keep its status codes. Business logic stays in Go.
- **Pages never call `useFetch` or `$fetch` directly.** They use a composable
  (`useAssets`, `useAsset`, `useMeta`) so the URL, query shape and defaults
  live in one place. Use `$fetch` only for user-triggered mutations, inside a
  composable or store action.
- **Server state lives in `useFetch`/`useAsyncData`, not in Pinia.** Pinia
  holds client state shared across pages (selected market, UI preferences,
  later the signed-in user). Stores are setup stores (`defineStore('id', () => {...})`).
- **Auth**: the Go API takes `Authorization: Bearer <session token>`. Only the
  Nuxt server sees the token: it keeps it in the HttpOnly, SameSite=Lax
  `shinrin_session` cookie (`server/utils/session.ts`) and `backendFetch`
  forwards it. `server/middleware/csrf.ts` refuses state-changing `/api`
  calls whose Origin is not this site. The signed-in user is client state in
  `useAuthStore`; pages that need one use `definePageMeta({ middleware: 'auth' })`.
  Every per-user repository call takes the user id, so another user's data
  is `ErrNotFound`, never readable.
- **Types**: API types live in `shared/types/api.ts` and must match the Go DTOs
  in `backend/internal/adapter/in/httpapi/dto.go`. Change both in the same PR.

### Components and style

- `<script setup lang="ts">`, `defineProps`/`defineModel` with types, no
  Options API.
- Components are presentational: they take props and emit events. Data
  fetching belongs to pages and composables.
- UI is **Nuxt UI v4** (Tailwind CSS v4) with the app's colors in
  `app/app.config.ts`. Prefer its components (`UCard`, `UTable`, `UAlert`,
  `UBadge`, ...) and Tailwind utilities over custom CSS; write class names in
  full (`text-success`, not `` `text-${color}` ``) so Tailwind generates them.
  Icons are Lucide (`i-lucide-*`), bundled from `@iconify-json/lucide`; fonts
  are system fonts, so the build downloads nothing.
- **Every user-facing string goes through i18n** (`@nuxtjs/i18n`, English and
  Portuguese in `i18n/locales/*.json`, keys kept in sync). Signals from Go are
  translated by their `code`, falling back to the English `message`. Numbers,
  money and dates use the helpers in `app/utils/format.ts` with the current
  locale; indicator display rules live in `app/utils/metrics.ts`.
- Charts use `lightweight-charts` inside a `.client.vue` component.
- A missing value is shown as missing ("No data", "—", a data-gaps note),
  never as zero and never silently dropped.
- The disclaimer is in the default layout and must stay on every page, and at
  the top of every AI report or suggestion. Any view that shows a score, fair
  value or timing view also shows `<AnalysisDisclaimer>` above it.

### Checks

`npm run lint` (ESLint via `@nuxt/eslint`), `npm run typecheck` and
`npm run build` must pass.

---

## Workflow

- Branch from `main`, open a PR, CI (`.github/workflows/ci.yml`) must be green.
- A PR that changes the API contract updates the Go DTOs, `shared/types/api.ts`
  and the handler tests together.
- A PR that changes these conventions updates this file.
