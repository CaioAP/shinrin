package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// SyncMarketPrices fills every tracked asset of src's market from a bulk
// source. The window starts at the earliest missing day across those assets,
// so one download serves them all; bars for untracked tickers are dropped.
func (s *Service) SyncMarketPrices(ctx context.Context, src port.MarketPriceSource) (Result, error) {
	assets, err := s.tracked(ctx, src.Market())
	if err != nil {
		return Result{}, err
	}
	if len(assets) == 0 {
		return Result{}, nil
	}

	want := make(map[domain.AssetKey]bool, len(assets))
	from := s.today()
	for _, a := range assets {
		want[a.Key] = true
		start, err := s.priceWindowStart(ctx, a.Key)
		if err != nil {
			return Result{}, err
		}
		if start.Before(from) {
			from = start
		}
	}
	to := s.today()
	if from.After(to) {
		return Result{Assets: len(assets)}, nil
	}

	// Fetch a calendar year at a time: a first backfill spans many yearly
	// files with every listed ticker, and only the tracked ones are kept.
	stored := 0
	for start := from; !start.After(to); start = time.Date(start.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC) {
		end := time.Date(start.Year(), 12, 31, 0, 0, 0, 0, time.UTC)
		if to.Before(end) {
			end = to
		}
		bars, err := src.MarketPrices(ctx, start, end)
		if err != nil {
			return Result{}, fmt.Errorf("%s prices %s..%s: %w", src.Name(), day(start), day(end), err)
		}
		keep := bars[:0]
		for _, b := range bars {
			if want[b.Asset] {
				keep = append(keep, b)
			}
		}
		if err := s.writePrices(ctx, keep); err != nil {
			return Result{}, fmt.Errorf("store %s prices: %w", src.Market(), err)
		}
		stored += len(keep)
	}
	s.opt.Logger.Info("market prices synced", "source", src.Name(), "from", day(from), "to", day(to), "bars", stored)
	return Result{Assets: len(assets), Rows: stored}, nil
}

// SyncPrices fills each tracked asset of market from a per-asset source,
// fetching only the days after its newest stored bar.
//
// When actions is not nil, the corporate actions for the same window are
// fetched and stored too. EOD vendors such as Tiingo return both in one
// response, so their adapter can serve the second call from the first.
func (s *Service) SyncPrices(ctx context.Context, market domain.Market, prices port.PriceSource, actions port.CorporateActionSource) (Result, error) {
	assets, err := s.tracked(ctx, market)
	if err != nil {
		return Result{}, err
	}
	to := s.today()
	return s.perAsset(ctx, "sync "+prices.Name()+" prices", assets, func(a domain.Asset) (int, error) {
		from, err := s.priceWindowStart(ctx, a.Key)
		if err != nil || from.After(to) {
			return 0, err
		}
		bars, err := prices.DailyPrices(ctx, a.Key, from, to)
		if err != nil {
			return 0, err
		}
		if err := s.writePrices(ctx, bars); err != nil {
			return 0, fmt.Errorf("store prices: %w", err)
		}
		n := len(bars)
		if actions != nil {
			acts, err := actions.CorporateActions(ctx, a.Key, from)
			if err != nil {
				return n, fmt.Errorf("corporate actions: %w", err)
			}
			if err := s.st.Actions.UpsertCorporateActions(ctx, acts); err != nil {
				return n, fmt.Errorf("store corporate actions: %w", err)
			}
			n += len(acts)
		}
		return n, nil
	})
}

// SyncCorporateActions refreshes dividends, JCP, splits and bonuses for every
// tracked asset of market. Use it for sources separate from the price feed
// (B3 publishes prices and corporate actions in different services).
func (s *Service) SyncCorporateActions(ctx context.Context, market domain.Market, src port.CorporateActionSource) (Result, error) {
	assets, err := s.tracked(ctx, market)
	if err != nil {
		return Result{}, err
	}
	return s.perAsset(ctx, "sync "+src.Name()+" corporate actions", assets, func(a domain.Asset) (int, error) {
		acts, err := src.CorporateActions(ctx, a.Key, s.opt.HistoryStart)
		if err != nil {
			return 0, err
		}
		if err := s.st.Actions.UpsertCorporateActions(ctx, acts); err != nil {
			return 0, fmt.Errorf("store: %w", err)
		}
		return len(acts), nil
	})
}

// SyncFundamentals refreshes reported fundamentals for every tracked asset of
// market. The first sync reads from HistoryStart; later ones re-read the last
// FundamentalsLookback before the newest stored period to catch
// restatements.
func (s *Service) SyncFundamentals(ctx context.Context, market domain.Market, src port.FundamentalsSource) (Result, error) {
	assets, err := s.tracked(ctx, market)
	if err != nil {
		return Result{}, err
	}
	return s.perAsset(ctx, "sync "+src.Name()+" fundamentals", assets, func(a domain.Asset) (int, error) {
		if a.Class == domain.ClassFII {
			return 0, domain.ErrNotFound // FIIs report monthly to a different CVM dataset
		}
		since := s.opt.HistoryStart
		last, ok, err := s.st.Fundamentals.LatestPeriodEnd(ctx, a.Key)
		if err != nil {
			return 0, fmt.Errorf("latest period: %w", err)
		}
		if ok {
			since = last.Add(-s.opt.FundamentalsLookback)
		}
		rows, err := src.Fundamentals(ctx, a.Key, since)
		if err != nil {
			return 0, err
		}
		for start := 0; start < len(rows); start += s.opt.BatchSize {
			if err := s.st.Fundamentals.UpsertFundamentals(ctx, rows[start:min(start+s.opt.BatchSize, len(rows))]); err != nil {
				return 0, fmt.Errorf("store: %w", err)
			}
		}
		return len(rows), nil
	})
}

func day(t time.Time) string { return t.Format(time.DateOnly) }
