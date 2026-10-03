// Package ingest moves data from source ports into the stores: the index
// universe, daily prices, corporate actions, fundamentals, quotes, news,
// macro series and government bonds. It decides what to fetch (only what is
// missing, for the assets Shinrin tracks) and leaves how to fetch it to the
// source adapters.
//
// Sources are passed per call rather than held by the Service, so the same
// code ingests B3 and US data from different providers (strategy pattern),
// chosen in cmd/shinrin/wire.go.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Stores are the repositories ingestion writes to.
type Stores struct {
	Assets       port.AssetRepository
	Prices       port.PriceRepository
	Fundamentals port.FundamentalRepository
	Actions      port.CorporateActionWriter
	Quotes       port.QuoteWriter
	News         port.NewsWriter
	Macro        port.MacroRepository
	Bonds        port.BondRepository
}

// Options tune ingestion. Zero values get defaults.
type Options struct {
	// HistoryStart is how far back the first sync of an asset goes.
	// Default: 2010-01-01, enough for 10-year averages and 3-year CAGR.
	HistoryStart time.Time
	// FundamentalsLookback is how far before the newest stored period a
	// repeat sync re-reads, to pick up restatements. Default: 18 months.
	FundamentalsLookback time.Duration
	// NewsLookback is how far back each news sync reads. Runs overlap and
	// news is deduplicated by URL. Default: 3 days.
	NewsLookback time.Duration
	// BatchSize caps rows per write. Default: 5000.
	BatchSize int
	Now       func() time.Time
	Logger    *slog.Logger
}

// Service runs ingestion against the stores.
type Service struct {
	st  Stores
	opt Options
}

// New builds the service.
func New(st Stores, opt Options) *Service {
	if opt.HistoryStart.IsZero() {
		opt.HistoryStart = time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if opt.FundamentalsLookback == 0 {
		opt.FundamentalsLookback = 18 * 30 * 24 * time.Hour
	}
	if opt.NewsLookback == 0 {
		opt.NewsLookback = 72 * time.Hour
	}
	if opt.BatchSize <= 0 {
		opt.BatchSize = 5000
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.Logger == nil {
		opt.Logger = slog.New(slog.DiscardHandler)
	}
	return &Service{st: st, opt: opt}
}

// Result summarises one sync.
type Result struct {
	Assets  int // assets considered
	Rows    int // rows written
	Skipped int // assets the source does not cover
	Failed  int // assets that errored
}

// SyncUniverse makes the index's current members the tracked assets of its
// market: members are inserted or refreshed, and assets that left the index
// keep their history but stop being index members.
func (s *Service) SyncUniverse(ctx context.Context, src port.UniverseSource, idx domain.Index) (Result, error) {
	members, err := src.Constituents(ctx, idx)
	if err != nil {
		return Result{}, fmt.Errorf("constituents of %s from %s: %w", idx, src.Name(), err)
	}
	if len(members) == 0 {
		// An empty list is far more likely a broken source than an empty
		// index; refuse rather than un-track the whole market.
		return Result{}, fmt.Errorf("constituents of %s from %s: %w: empty list", idx, src.Name(), domain.ErrInvalid)
	}

	existing, err := s.st.Assets.ListAssets(ctx, port.AssetFilter{Market: idx.Market()})
	if err != nil {
		return Result{}, fmt.Errorf("list %s assets: %w", idx.Market(), err)
	}
	known := make(map[domain.AssetKey]domain.Asset, len(existing))
	for _, a := range existing {
		known[a.Key] = a
	}

	upserts := make([]domain.Asset, 0, len(members))
	inIndex := make(map[domain.AssetKey]bool, len(members))
	for _, m := range members {
		inIndex[m.Key] = true
		m.IndexMember, m.Active = true, true
		upserts = append(upserts, mergeAsset(known[m.Key], m))
	}
	for _, a := range existing {
		if a.IndexMember && !inIndex[a.Key] && a.Class != domain.ClassIndex {
			a.IndexMember = false
			upserts = append(upserts, a)
		}
	}
	if err := s.st.Assets.UpsertAssets(ctx, upserts); err != nil {
		return Result{}, fmt.Errorf("upsert %s members: %w", idx, err)
	}
	s.opt.Logger.Info("universe synced", "index", idx, "source", src.Name(), "members", len(members), "left", len(upserts)-len(members))
	return Result{Assets: len(members), Rows: len(upserts)}, nil
}

// mergeAsset keeps identifiers a previous source found when the new one does
// not know them (the S&P list has CIKs, the B3 list has no CNPJ, ...).
func mergeAsset(old, upd domain.Asset) domain.Asset {
	keep := func(dst *string, v string) {
		if *dst == "" {
			*dst = v
		}
	}
	keep(&upd.Name, old.Name)
	keep(&upd.Sector, old.Sector)
	keep(&upd.ISIN, old.ISIN)
	keep(&upd.CIK, old.CIK)
	keep(&upd.CNPJ, old.CNPJ)
	return upd
}

// tracked lists the active assets of a market that a price, quote, news or
// filing source should cover. Index rows and government bonds have their own
// routines.
func (s *Service) tracked(ctx context.Context, m domain.Market) ([]domain.Asset, error) {
	all, err := s.st.Assets.ListAssets(ctx, port.AssetFilter{Market: m})
	if err != nil {
		return nil, fmt.Errorf("list %s assets: %w", m, err)
	}
	out := all[:0]
	for _, a := range all {
		if a.Active && a.Class != domain.ClassIndex && a.Class != domain.ClassGovBond {
			out = append(out, a)
		}
	}
	return out, nil
}

// today is the end of every fetch window, in UTC calendar terms.
func (s *Service) today() time.Time {
	return truncateDay(s.opt.Now())
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// priceWindowStart is the first missing day for an asset.
func (s *Service) priceWindowStart(ctx context.Context, key domain.AssetKey) (time.Time, error) {
	last, ok, err := s.st.Prices.LatestPriceDate(ctx, key)
	if err != nil {
		return time.Time{}, fmt.Errorf("latest price of %s: %w", key, err)
	}
	if !ok {
		return s.opt.HistoryStart, nil
	}
	return truncateDay(last).AddDate(0, 0, 1), nil
}

func (s *Service) writePrices(ctx context.Context, bars []domain.PriceBar) error {
	for start := 0; start < len(bars); start += s.opt.BatchSize {
		end := min(start+s.opt.BatchSize, len(bars))
		if err := s.st.Prices.UpsertPrices(ctx, bars[start:end]); err != nil {
			return err
		}
	}
	return nil
}

// perAsset runs fn for every asset, isolating failures: one delisted ticker
// must not stop the other 500. Assets the source does not cover
// (domain.ErrNotFound) are skipped quietly; any other failure is counted and
// returned joined, so the scheduler retries the routine.
func (s *Service) perAsset(ctx context.Context, op string, assets []domain.Asset, fn func(domain.Asset) (int, error)) (Result, error) {
	r := Result{Assets: len(assets)}
	var errs []error
	for _, a := range assets {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		n, err := fn(a)
		r.Rows += n
		switch {
		case err == nil:
		case errors.Is(err, domain.ErrNotFound):
			r.Skipped++
			s.opt.Logger.Debug(op+": not covered", "asset", a.Key.String(), "err", err)
		default:
			r.Failed++
			errs = append(errs, fmt.Errorf("%s: %w", a.Key, err))
			s.opt.Logger.Warn(op+" failed", "asset", a.Key.String(), "err", err)
		}
	}
	s.opt.Logger.Info(op+" done", "assets", r.Assets, "rows", r.Rows, "skipped", r.Skipped, "failed", r.Failed)
	if len(errs) > 0 {
		return r, fmt.Errorf("%s: %d of %d assets failed: %w", op, r.Failed, r.Assets, errors.Join(errs...))
	}
	return r, nil
}
