package main

import (
	"context"
	"errors"
	"net/http"
	"time"
)

func runAPI(ctx context.Context, c *container) error {
	srv := &http.Server{
		Addr:              c.cfg.HTTPAddr,
		Handler:           c.httpHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), c.cfg.ShutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	c.log.Info("api listening", "addr", c.cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
