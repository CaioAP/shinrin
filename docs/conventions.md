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
  cmd/shinrin/          composition root: config, wiring (wire.go), `api` and `worker` roles
  internal/
    domain/             entities, value objects, domain errors. Standard library only.
    port/               interfaces: driving.go (use cases), driven.go (what the app needs)
    app/<area>/         application services, one package per use-case area
    adapter/in/<name>/  driving adapters: httpapi (REST), later the River job handlers
    adapter/out/<name>/ driven adapters: memory, later postgres, b3cotahist, finnhub, anthropic, ...
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
  `CatalogService`, later `IngestService`, `ScoringService`, `ReportService`.
- **Driven ports** (`port/driven.go`) are the outside world: repositories
  (`AssetReader`, `AssetWriter`), market data (`PriceSource`, `QuoteSource`,
  `FundamentalsSource`, `CorporateActionSource`, `NewsSource`, `MacroSource`),
  AI (`LLMProvider`), infrastructure (`HealthChecker`, `Routine`).
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
  (for example `b3cotahist` implements `PriceSource`; `finnhub` implements
  `QuoteSource` and `NewsSource`). Rate limits, retries and parsing belong
  inside the adapter.
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
| Strategy | `PriceSource`, `LLMProvider`, later scoring factors | pick an implementation per market, provider or asset class at wiring time |
| Decorator | HTTP middleware; later caching, rate-limited or retrying sources | add behaviour without touching the wrapped type |
| Composite | `system.Service` aggregates `HealthChecker`s | one health answer from many dependencies |
| Repository | `AssetReader` / `AssetWriter` | storage behind an interface |
| Registry | `routines()` in `cmd/shinrin/worker.go` | the worker discovers routines from one list |

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
- **Types**: API types live in `shared/types/api.ts` and must match the Go DTOs
  in `backend/internal/adapter/in/httpapi/dto.go`. Change both in the same PR.

### Components and style

- `<script setup lang="ts">`, `defineProps`/`defineModel` with types, no
  Options API.
- Components are presentational: they take props and emit events. Data
  fetching belongs to pages and composables.
- Scoped styles. A component library (Nuxt UI or shadcn-vue) is chosen in the
  web app phase; until then keep CSS minimal.
- The disclaimer is in the default layout and must stay on every page, and at
  the top of every AI report or suggestion.

### Checks

`npm run lint` (ESLint via `@nuxt/eslint`) and `npm run typecheck` must pass.

---

## Workflow

- Branch from `main`, open a PR, CI (`.github/workflows/ci.yml`) must be green.
- A PR that changes the API contract updates the Go DTOs, `shared/types/api.ts`
  and the handler tests together.
- A PR that changes these conventions updates this file.
