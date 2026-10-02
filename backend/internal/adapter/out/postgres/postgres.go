// Package postgres stores Shinrin's data in PostgreSQL with pgx. Store
// implements every repository port (assets, prices, fundamentals, corporate
// actions, indicators) and port.HealthChecker; consumers still depend on the
// narrow port they need. Schema changes are goose migrations embedded from
// migrations/.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/CaioAP/shinrin/backend/internal/port"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store is the Postgres implementation of the repository ports.
type Store struct {
	pool *pgxpool.Pool
}

var (
	_ port.AssetRepository           = (*Store)(nil)
	_ port.PriceRepository           = (*Store)(nil)
	_ port.FundamentalRepository     = (*Store)(nil)
	_ port.CorporateActionRepository = (*Store)(nil)
	_ port.IndicatorReader           = (*Store)(nil)
	_ port.IndicatorWriter           = (*Store)(nil)
	_ port.HealthChecker             = (*Store)(nil)
)

// Open connects a pool to url (a postgres:// URL or DSN).
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return pool, nil
}

// New wraps an open pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Migrate applies pending schema migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, sub)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Name implements port.HealthChecker.
func (s *Store) Name() string { return "postgres" }

// Check implements port.HealthChecker.
func (s *Store) Check(ctx context.Context) error { return s.pool.Ping(ctx) }
