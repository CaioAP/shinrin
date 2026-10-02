package main

import (
	"context"
	"log/slog"

	"github.com/CaioAP/shinrin/backend/internal/port"
)

// routines returns every scheduled routine. The data pipeline phase adds the
// ingestion routines here and a River adapter to schedule them.
func routines() []port.Routine {
	return nil
}

func runWorker(ctx context.Context, log *slog.Logger) error {
	rs := routines()
	names := make([]string, len(rs))
	for i, r := range rs {
		names[i] = r.Name()
	}
	log.Info("worker started", "routines", names)
	<-ctx.Done()
	return nil
}
