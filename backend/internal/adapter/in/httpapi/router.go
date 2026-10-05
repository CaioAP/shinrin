// Package httpapi is the driving HTTP adapter: it exposes the REST API the
// Nuxt app consumes and translates each request into a call on a driving port.
// Handlers hold no business logic; they parse, call a port, and render.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Deps are the driving ports the API needs. Add a field per new use case.
type Deps struct {
	System   port.SystemService
	Catalog  port.CatalogService
	Analysis port.AnalysisService
	Market   port.MarketService
	Accounts port.AccountService
	Lists    port.WatchlistService
	// Credentials and Reports serve users' own LLM keys and AI reports.
	Credentials port.CredentialService
	Reports     port.UserReportService
	Logger      *slog.Logger
	// Now is the clock for relative ranges; nil means time.Now.
	Now func() time.Time
}

// NewRouter builds the API routes wrapped in the standard middleware chain.
func NewRouter(d Deps) http.Handler {
	sys := systemHandler{svc: d.System}
	cat := catalogHandler{svc: d.Catalog}
	an := analysisHandler{svc: d.Analysis}
	now := d.Now
	if now == nil {
		now = time.Now
	}
	mk := marketHandler{svc: d.Market, now: now}
	au := authHandler{svc: d.Accounts}
	wl := watchlistHandler{svc: d.Lists}
	ai := aiHandler{creds: d.Credentials, reports: d.Reports}
	user := au.requireUser

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", sys.health)
	mux.HandleFunc("GET /api/v1/meta", sys.meta)
	mux.HandleFunc("GET /api/v1/assets", cat.list)
	mux.HandleFunc("GET /api/v1/assets/{market}/{symbol}", cat.get)
	mux.HandleFunc("GET /api/v1/assets/{market}/{symbol}/analysis", an.analyze)
	mux.HandleFunc("GET /api/v1/assets/{market}/{symbol}/prices", mk.prices)
	mux.HandleFunc("GET /api/v1/assets/{market}/{symbol}/dividends", mk.dividends)
	mux.HandleFunc("GET /api/v1/assets/{market}/{symbol}/news", mk.news)
	mux.HandleFunc("GET /api/v1/macro", mk.macro)
	mux.HandleFunc("GET /api/v1/rankings", an.rank)
	mux.HandleFunc("GET /api/v1/outlook", an.outlook)
	mux.HandleFunc("GET /api/v1/risk-questionnaire", questionnaire)

	mux.HandleFunc("POST /api/v1/auth/signup", au.signUp)
	mux.HandleFunc("POST /api/v1/auth/signin", au.signIn)
	mux.HandleFunc("POST /api/v1/auth/signout", au.signOut)
	mux.HandleFunc("GET /api/v1/me", user(au.me))
	mux.HandleFunc("PUT /api/v1/me/risk-profile", user(au.setRiskProfile))
	mux.HandleFunc("DELETE /api/v1/me", user(au.deleteAccount))

	mux.HandleFunc("GET /api/v1/watchlists", user(wl.list))
	mux.HandleFunc("POST /api/v1/watchlists", user(wl.create))
	mux.HandleFunc("GET /api/v1/watchlists/{id}", user(wl.get))
	mux.HandleFunc("PATCH /api/v1/watchlists/{id}", user(wl.rename))
	mux.HandleFunc("DELETE /api/v1/watchlists/{id}", user(wl.remove))
	mux.HandleFunc("PUT /api/v1/watchlists/{id}/items/{market}/{symbol}", user(wl.addItem))
	mux.HandleFunc("DELETE /api/v1/watchlists/{id}/items/{market}/{symbol}", user(wl.removeItem))

	mux.HandleFunc("GET /api/v1/me/llm", user(ai.settings))
	mux.HandleFunc("PUT /api/v1/me/llm", user(ai.save))
	mux.HandleFunc("DELETE /api/v1/me/llm", user(ai.remove))
	mux.HandleFunc("POST /api/v1/me/llm/test", user(ai.test))
	mux.HandleFunc("GET /api/v1/assets/{market}/{symbol}/reports", user(ai.list))
	mux.HandleFunc("POST /api/v1/assets/{market}/{symbol}/reports", user(ai.generate))
	mux.HandleFunc("GET /api/v1/reports/{id}", user(ai.get))

	return chain(mux, recoverer(d.Logger), requestLogger(d.Logger))
}
