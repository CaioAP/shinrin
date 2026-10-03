// Package httpapi is the driving HTTP adapter: it exposes the REST API the
// Nuxt app consumes and translates each request into a call on a driving port.
// Handlers hold no business logic; they parse, call a port, and render.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Deps are the driving ports the API needs. Add a field per new use case.
type Deps struct {
	System   port.SystemService
	Catalog  port.CatalogService
	Analysis port.AnalysisService
	Logger   *slog.Logger
}

// NewRouter builds the API routes wrapped in the standard middleware chain.
func NewRouter(d Deps) http.Handler {
	sys := systemHandler{svc: d.System}
	cat := catalogHandler{svc: d.Catalog}
	an := analysisHandler{svc: d.Analysis}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", sys.health)
	mux.HandleFunc("GET /api/v1/meta", sys.meta)
	mux.HandleFunc("GET /api/v1/assets", cat.list)
	mux.HandleFunc("GET /api/v1/assets/{market}/{symbol}", cat.get)
	mux.HandleFunc("GET /api/v1/assets/{market}/{symbol}/analysis", an.analyze)
	mux.HandleFunc("GET /api/v1/rankings", an.rank)
	mux.HandleFunc("GET /api/v1/outlook", an.outlook)

	return chain(mux, recoverer(d.Logger), requestLogger(d.Logger))
}
