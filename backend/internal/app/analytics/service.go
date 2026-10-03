// Package analytics turns stored prices, corporate actions and fundamentals
// into indicator snapshots (technicals and valuation ratios) for every
// tracked asset. The maths lives in domain/indicators; this service only
// loads the inputs and stores the results.
package analytics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Stores are the repositories the service reads and writes.
type Stores struct {
	Assets       port.AssetReader
	Prices       port.PriceReader
	Fundamentals port.FundamentalReader
	Actions      port.CorporateActionReader
	Indicators   port.IndicatorWriter
}

// Service computes indicators.
type Service struct {
	st     Stores
	now    func() time.Time
	logger *slog.Logger
}

// New builds the service. now and logger may be nil.
func New(st Stores, now func() time.Time, logger *slog.Logger) *Service {
	if now == nil {
		now = time.Now
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Service{st: st, now: now, logger: logger}
}

// Windows of history each snapshot reads. Prices: 252 trading days plus room
// for the 200-day average and holidays. Fundamentals: enough quarters to
// derive a fourth quarter and a TTM sum.
const (
	priceLookback        = 400 * 24 * time.Hour
	fundamentalsLookback = 3 * 365 * 24 * time.Hour
)

// ComputeMarket computes and stores a snapshot for every active asset of m.
// Index rows are skipped; they have no filings.
// It returns how many snapshots were written.
func (s *Service) ComputeMarket(ctx context.Context, m domain.Market) (int, error) {
	assets, err := s.st.Assets.ListAssets(ctx, port.AssetFilter{Market: m})
	if err != nil {
		return 0, fmt.Errorf("list %s assets: %w", m, err)
	}
	var (
		sets []domain.IndicatorSet
		errs []error
	)
	for _, a := range assets {
		if !a.Active || a.Class == domain.ClassIndex {
			continue
		}
		set, ok, err := s.Compute(ctx, a)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if ok {
			sets = append(sets, set)
		}
	}
	if len(sets) > 0 {
		if err := s.st.Indicators.UpsertIndicators(ctx, sets); err != nil {
			return 0, fmt.Errorf("store indicators: %w", err)
		}
	}
	s.logger.Info("indicators computed", "market", m, "assets", len(sets), "failed", len(errs))
	return len(sets), errors.Join(errs...)
}

// Compute builds one asset's snapshot as of its latest bar. ok is false when
// the asset has no prices yet.
func (s *Service) Compute(ctx context.Context, a domain.Asset) (domain.IndicatorSet, bool, error) {
	key := a.Key
	now := s.now()
	bars, err := s.st.Prices.PricesBetween(ctx, key, now.Add(-priceLookback), now)
	if err != nil {
		return domain.IndicatorSet{}, false, fmt.Errorf("prices of %s: %w", key, err)
	}
	if len(bars) == 0 {
		return domain.IndicatorSet{}, false, nil
	}
	actions, err := s.st.Actions.CorporateActionsSince(ctx, key, bars[0].Date)
	if err != nil {
		return domain.IndicatorSet{}, false, fmt.Errorf("corporate actions of %s: %w", key, err)
	}
	funds, err := s.st.Fundamentals.FundamentalsSince(ctx, key, now.Add(-fundamentalsLookback))
	if err != nil {
		return domain.IndicatorSet{}, false, fmt.Errorf("fundamentals of %s: %w", key, err)
	}
	if a.Class == domain.ClassStock && key.IsB3Unit() {
		funds = withoutMetric(funds, domain.MetricShares) // see IsB3Unit
	}
	set, ok := indicators.Snapshot(key, bars, funds, actions)
	return set, ok, nil
}

func withoutMetric(rows []domain.Fundamental, metric string) []domain.Fundamental {
	out := rows[:0:0]
	for _, r := range rows {
		if r.Metric != metric {
			out = append(out, r)
		}
	}
	return out
}

type routine struct {
	name string
	run  func(context.Context) error
}

func (r routine) Name() string                  { return r.name }
func (r routine) Run(ctx context.Context) error { return r.run(ctx) }

// Routine computes indicators for the given markets.
func (s *Service) Routine(name string, markets ...domain.Market) port.Routine {
	return routine{name, func(ctx context.Context) error {
		var errs []error
		for _, m := range markets {
			if _, err := s.ComputeMarket(ctx, m); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}}
}
