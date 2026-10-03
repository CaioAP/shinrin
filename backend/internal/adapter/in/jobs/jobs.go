// Package jobs runs routines on a schedule with River, a Postgres-backed job
// queue: each run is a row in river_job, retried with backoff on failure and
// visible for a status page, and only one run of a routine is queued or
// running at a time even with several workers. It is a driving adapter: the
// scheduler calls the application's port.Routine values, and it knows
// nothing about what they do.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/robfig/cron/v3"

	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Entry schedules one routine.
type Entry struct {
	Routine port.Routine
	// Schedule is a five-field cron expression, optionally prefixed with a
	// time zone: "CRON_TZ=America/Sao_Paulo 30 20 * * 1-5".
	Schedule string
	// Queue groups routines that share a provider's rate limit; each queue
	// runs one job at a time. Default: "default".
	Queue string
	// Timeout bounds one run. Routines are incremental, so a run cut short
	// resumes where it stopped on the next attempt. Default: 2h.
	Timeout time.Duration
}

// Scheduler owns the River client.
type Scheduler struct {
	client *river.Client[pgx.Tx]
	log    *slog.Logger
}

const maxAttempts = 4

// New validates entries and builds a River client over pool.
func New(pool *pgxpool.Pool, entries []Entry, log *slog.Logger) (*Scheduler, error) {
	w := &worker{routines: map[string]Entry{}}
	queues := map[string]river.QueueConfig{}
	var periodic []*river.PeriodicJob

	for _, e := range entries {
		name := e.Routine.Name()
		if _, dup := w.routines[name]; dup {
			return nil, fmt.Errorf("routine %q scheduled twice", name)
		}
		sched, err := cron.ParseStandard(e.Schedule)
		if err != nil {
			return nil, fmt.Errorf("routine %q: schedule %q: %w", name, e.Schedule, err)
		}
		if e.Queue == "" {
			e.Queue = river.QueueDefault
		}
		if e.Timeout <= 0 {
			e.Timeout = 2 * time.Hour
		}
		w.routines[name] = e
		queues[e.Queue] = river.QueueConfig{MaxWorkers: 1}
		periodic = append(periodic, river.NewPeriodicJob(sched, func() (river.JobArgs, *river.InsertOpts) {
			return routineArgs{Name: name}, insertOpts(e.Queue)
		}, &river.PeriodicJobOpts{ID: name}))
	}

	workers := river.NewWorkers()
	if err := river.AddWorkerSafely(workers, w); err != nil {
		return nil, err
	}
	if len(queues) == 0 {
		queues[river.QueueDefault] = river.QueueConfig{MaxWorkers: 1}
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:       queues,
		Workers:      workers,
		PeriodicJobs: periodic,
		MaxAttempts:  maxAttempts,
		Logger:       log,
	})
	if err != nil {
		return nil, fmt.Errorf("river client: %w", err)
	}
	return &Scheduler{client: client, log: log}, nil
}

// Run starts working jobs and blocks until ctx is cancelled, then stops
// gracefully, letting running routines finish within stopTimeout.
func (s *Scheduler) Run(ctx context.Context, stopTimeout time.Duration) error {
	if err := s.client.Start(ctx); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	if err := s.client.Stop(stopCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}

// Enqueue asks for a routine to run now, outside its schedule. It is a
// no-op when a run is already queued or running.
func (s *Scheduler) Enqueue(ctx context.Context, name, queue string) error {
	if queue == "" {
		queue = river.QueueDefault
	}
	_, err := s.client.Insert(ctx, routineArgs{Name: name}, insertOpts(queue))
	return err
}

// Migrate creates or upgrades River's tables.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	m, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	if _, err := m.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}
	return nil
}

func insertOpts(queue string) *river.InsertOpts {
	return &river.InsertOpts{
		Queue:       queue,
		MaxAttempts: maxAttempts,
		// One pending or running job per routine: a slow run never stacks
		// up behind itself.
		UniqueOpts: river.UniqueOpts{ByArgs: true},
	}
}

// routineArgs is the job payload: just the routine's name.
type routineArgs struct {
	Name string `json:"name" river:"unique"`
}

// Kind implements river.JobArgs.
func (routineArgs) Kind() string { return "routine" }

// worker runs whichever routine a job names (registry pattern: one River
// worker for every routine).
type worker struct {
	river.WorkerDefaults[routineArgs]
	routines map[string]Entry
}

func (w *worker) Work(ctx context.Context, job *river.Job[routineArgs]) error {
	e, ok := w.routines[job.Args.Name]
	if !ok {
		// A job for a routine that was removed from the registry: retrying
		// cannot help.
		return river.JobCancel(fmt.Errorf("unknown routine %q", job.Args.Name))
	}
	return e.Routine.Run(ctx)
}

func (w *worker) Timeout(job *river.Job[routineArgs]) time.Duration {
	if e, ok := w.routines[job.Args.Name]; ok {
		return e.Timeout
	}
	return time.Minute
}
