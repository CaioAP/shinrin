// Command shinrin runs the Shinrin backend.
//
// Usage:
//
//	shinrin api      serve the HTTP API used by the Nuxt app
//	shinrin worker   run scheduled ingestion, scoring and AI report jobs
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

	"github.com/CaioAP/shinrin/backend/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: shinrin <api|worker>")
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

	switch os.Args[1] {
	case "api":
		err = runAPI(ctx, cfg, log)
	case "worker":
		err = runWorker(ctx, log)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		log.Error("shinrin exited", "err", err)
		os.Exit(1)
	}
}
