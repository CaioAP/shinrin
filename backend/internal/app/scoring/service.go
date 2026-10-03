// Package scoring runs the analysis engine over the tracked universe: it
// loads each asset's latest indicators, dividend history and recent news,
// asks domain/scoring for peer-relative factor scores, and stores the
// scorecards. The rules live in domain/scoring; this service only moves
// data in and out.
package scoring

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	engine "github.com/CaioAP/shinrin/backend/internal/domain/scoring"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Stores are the repositories the service reads and writes.
type Stores struct {
	Assets     port.AssetReader
	Indicators port.IndicatorReader
	Actions    port.CorporateActionReader
	News       port.NewsReader
	Scores     port.ScoreWriter
}

// Service scores the universe.
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

// Windows each scorecard reads: five years of dividends for consistency, a
// month of news for sentiment.
const (
	dividendLookback = 5
	newsWindow       = 30 * 24 * time.Hour
	newsLimit        = 100
)

// ScoreMarket scores every active stock, FII and REIT of m that has
// indicators, ranking each against its peers, and stores the scorecards. It
// returns how many were written.
func (s *Service) ScoreMarket(ctx context.Context, m domain.Market) (int, error) {
	assets, err := s.st.Assets.ListAssets(ctx, port.AssetFilter{Market: m})
	if err != nil {
		return 0, fmt.Errorf("list %s assets: %w", m, err)
	}
	var (
		inputs []engine.Input
		errs   []error
	)
	for _, a := range assets {
		if _, scored := engine.ModelFor(a); !a.Active || !scored {
			continue
		}
		in, ok, err := s.input(ctx, a)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if ok {
			inputs = append(inputs, in)
		}
	}
	cards := engine.Score(inputs)
	if len(cards) > 0 {
		if err := s.st.Scores.UpsertScorecards(ctx, cards); err != nil {
			return 0, fmt.Errorf("store scorecards: %w", err)
		}
	}
	s.logger.Info("assets scored", "market", m, "assets", len(cards), "failed", len(errs))
	return len(cards), errors.Join(errs...)
}

// input gathers one asset's metrics. ok is false when it has no indicators
// yet (no prices synced).
func (s *Service) input(ctx context.Context, a domain.Asset) (engine.Input, bool, error) {
	set, err := s.st.Indicators.LatestIndicators(ctx, a.Key)
	if errors.Is(err, domain.ErrNotFound) {
		return engine.Input{}, false, nil
	}
	if err != nil {
		return engine.Input{}, false, fmt.Errorf("indicators of %s: %w", a.Key, err)
	}
	actions, err := s.st.Actions.CorporateActionsSince(ctx, a.Key, set.AsOf.AddDate(-dividendLookback, 0, 0))
	if err != nil {
		return engine.Input{}, false, fmt.Errorf("corporate actions of %s: %w", a.Key, err)
	}
	news, err := s.st.News.NewsFor(ctx, a.Key, s.now().Add(-newsWindow), newsLimit)
	if err != nil {
		return engine.Input{}, false, fmt.Errorf("news of %s: %w", a.Key, err)
	}
	extra := map[string]float64{engine.DividendYears: engine.CountDividendYears(actions, set.AsOf)}
	if tone, n, ok := engine.NewsToneOf(news); ok {
		extra[engine.NewsTone] = tone
		extra[engine.NewsCount] = float64(n)
	}
	return engine.Input{Asset: a, AsOf: set.AsOf, Metrics: engine.Metrics(set.Values, extra)}, true, nil
}

type routine struct {
	name string
	run  func(context.Context) error
}

func (r routine) Name() string                  { return r.name }
func (r routine) Run(ctx context.Context) error { return r.run(ctx) }

// Routine scores the given markets.
func (s *Service) Routine(name string, markets ...domain.Market) port.Routine {
	return routine{name, func(ctx context.Context) error {
		var errs []error
		for _, m := range markets {
			if _, err := s.ScoreMarket(ctx, m); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}}
}
