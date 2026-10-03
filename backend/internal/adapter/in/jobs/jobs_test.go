package jobs_test

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/jobs"
)

type routine struct {
	name string
	ran  chan struct{}
}

func (r routine) Name() string { return r.name }
func (r routine) Run(context.Context) error {
	r.ran <- struct{}{}
	return nil
}

func TestNewValidatesEntries(t *testing.T) {
	r := routine{name: "a"}
	tests := []struct {
		name    string
		entries []jobs.Entry
		want    string
	}{
		{"bad cron", []jobs.Entry{{Routine: r, Schedule: "every day"}}, "schedule"},
		{"duplicate", []jobs.Entry{{Routine: r, Schedule: "0 * * * *"}, {Routine: r, Schedule: "5 * * * *"}}, "twice"},
		{"bad zone", []jobs.Entry{{Routine: r, Schedule: "CRON_TZ=Mars/Olympus 0 * * * *"}}, "schedule"},
	}
	for _, tt := range tests {
		_, err := jobs.New(nil, tt.entries, slog.New(slog.DiscardHandler))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want it to mention %q", tt.name, err, tt.want)
		}
	}
}

func TestEnqueueRunsRoutine(t *testing.T) {
	url := os.Getenv("SHINRIN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SHINRIN_TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("jobs_%d", rand.Uint64())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	pool, err := pgxpool.New(ctx, url+sep+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := jobs.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	r := routine{name: "sync_test", ran: make(chan struct{}, 1)}
	s, err := jobs.New(pool, []jobs.Entry{{Routine: r, Schedule: "CRON_TZ=America/Sao_Paulo 30 20 * * 1-5", Queue: "b3"}}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- s.Run(runCtx, 5*time.Second) }()

	if err := s.Enqueue(ctx, "sync_test", "b3"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-r.ran:
	case <-ctx.Done():
		t.Fatal("routine did not run")
	}
	stop()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
