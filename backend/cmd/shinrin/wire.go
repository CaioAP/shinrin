package main

import (
	"log/slog"
	"net/http"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/httpapi"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/catalog"
	"github.com/CaioAP/shinrin/backend/internal/app/system"
	"github.com/CaioAP/shinrin/backend/internal/config"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// container holds the wired application. Both roles build from it, so the api
// and the worker always share the same adapters.
type container struct {
	assets  port.AssetRepository
	system  port.SystemService
	catalog port.CatalogService
}

// build is the manual dependency injection for the whole app. Swap an adapter
// here (for example memory -> postgres) and nothing else changes.
func build(cfg config.Config) *container {
	// Driven adapters.
	assets := memory.NewAssetRepository() // replaced by a Postgres adapter in the data pipeline phase

	// Application services.
	return &container{
		assets:  assets,
		system:  system.New(cfg.Version),
		catalog: catalog.New(assets),
	}
}

func (c *container) httpHandler(log *slog.Logger) http.Handler {
	return httpapi.NewRouter(httpapi.Deps{
		System:  c.system,
		Catalog: c.catalog,
		Logger:  log,
	})
}
