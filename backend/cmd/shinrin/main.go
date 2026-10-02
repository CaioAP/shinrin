// Command shinrin runs the Shinrin backend.
//
// Usage:
//
//	shinrin api                 serve the HTTP API used by the Nuxt app
//	shinrin worker              run scheduled ingestion and analysis routines
//	shinrin migrate             create or upgrade the database schema
//	shinrin run <routine>...    run routines once, now (backfills, debugging)
//
// This package is the composition root: it is the only place that knows every
// concrete adapter. It reads config, builds the adapters, injects them into
// the application services, and hands those to the driving adapters. See
// docs/conventions.md.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/jobs"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/postgres"
	"github.com/CaioAP/shinrin/backend/internal/config"
)

const usage = "usage: shinrin <api|worker|migrate|run routine...>"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(2)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, log, os.Args[1], os.Args[2:]); err != nil {
		log.Error("shinrin exited", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, log *slog.Logger, cmd string, args []string) error {
	switch cmd {
	case "api", "worker", "migrate", "run":
	default:
		return fmt.Errorf("unknown command %q; %s", cmd, usage)
	}
	c, cleanup, err := build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer cleanup()

	switch cmd {
	case "api":
		return runAPI(ctx, c)
	case "worker":
		return runWorker(ctx, c)
	case "migrate":
		return migrate(ctx, c)
	default:
		return runRoutines(ctx, c, args)
	}
}

// migrate applies Shinrin's schema and River's job tables.
func migrate(ctx context.Context, c *container) error {
	if c.pool == nil {
		return errNoDatabase
	}
	if err := postgres.Migrate(ctx, c.pool); err != nil {
		return err
	}
	if err := jobs.Migrate(ctx, c.pool); err != nil {
		return err
	}
	c.log.Info("migrations applied")
	return nil
}
