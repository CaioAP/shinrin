// Command shinrin runs the Shinrin backend.
//
// Usage:
//
//	shinrin api      serve the HTTP API used by the Nuxt app
//	shinrin worker   run scheduled ingestion, scoring and AI report jobs
//
// See docs/design.md for the architecture.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/httpapi"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: shinrin <api|worker>")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "api":
		err = runAPI(ctx)
	case "worker":
		err = runWorker(ctx)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		slog.Error("shinrin exited", "err", err)
		os.Exit(1)
	}
}

func runAPI(ctx context.Context) error {
	addr := envOr("SHINRIN_ADDR", ":8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("api listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runWorker(ctx context.Context) error {
	// The River job queue and routines are built in the data pipeline phase.
	slog.Info("worker started (no jobs registered yet)")
	<-ctx.Done()
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
