package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// SyncQuotes refreshes the latest quote of every tracked asset of a market.
// Quotes are a cache of "now", so there is no window: each run overwrites.
func (s *Service) SyncQuotes(ctx context.Context, m domain.Market, src port.QuoteSource) (Result, error) {
	assets, err := s.tracked(ctx, m)
	if err != nil {
		return Result{}, err
	}
	keys := make([]domain.AssetKey, len(assets))
	for i, a := range assets {
		keys[i] = a.Key
	}
	r := Result{Assets: len(keys)}
	if len(keys) == 0 {
		return r, nil
	}
	quotes, fetchErr := src.Quotes(ctx, keys)
	// Keep what arrived before a mid-run failure (a rate limit on asset 300
	// should not discard the first 299 quotes), then report the failure.
	if len(quotes) > 0 {
		if err := s.st.Quotes.UpsertQuotes(ctx, quotes); err != nil {
			return r, fmt.Errorf("store %s quotes: %w", m, err)
		}
	}
	r.Rows = len(quotes)
	r.Skipped = len(keys) - len(quotes)
	s.opt.Logger.Info("quotes done", "market", m, "source", src.Name(), "assets", r.Assets, "rows", r.Rows)
	if fetchErr != nil {
		return r, fmt.Errorf("%s quotes from %s: %w", m, src.Name(), fetchErr)
	}
	return r, nil
}

// SyncCompanyNews fetches recent news per tracked asset. News is deduplicated
// by URL on write, so the lookback window can overlap the previous run.
func (s *Service) SyncCompanyNews(ctx context.Context, m domain.Market, src port.CompanyNewsSource) (Result, error) {
	assets, err := s.tracked(ctx, m)
	if err != nil {
		return Result{}, err
	}
	since := s.opt.Now().Add(-s.opt.NewsLookback)
	return s.perAsset(ctx, src.Name()+" news", assets, func(a domain.Asset) (int, error) {
		items, err := src.CompanyNews(ctx, a.Key, since)
		if err != nil || len(items) == 0 {
			return 0, err
		}
		return len(items), s.st.News.UpsertNews(ctx, items)
	})
}

// SyncNews fetches market-wide news (CVM material facts), already tagged
// with the assets each item concerns. lookback overrides
// Options.NewsLookback for sources published in batches (CVM's IPE dataset
// is refreshed weekly); zero keeps the default.
func (s *Service) SyncNews(ctx context.Context, src port.NewsSource, lookback time.Duration) (Result, error) {
	if lookback == 0 {
		lookback = s.opt.NewsLookback
	}
	items, err := src.News(ctx, s.opt.Now().Add(-lookback))
	if err != nil {
		return Result{}, fmt.Errorf("news from %s: %w", src.Name(), err)
	}
	if err := s.st.News.UpsertNews(ctx, items); err != nil {
		return Result{}, fmt.Errorf("store news from %s: %w", src.Name(), err)
	}
	s.opt.Logger.Info("news done", "source", src.Name(), "rows", len(items))
	return Result{Rows: len(items)}, nil
}

// SyncMacro fills every series src serves from the day after its newest
// stored observation (or HistoryStart).
func (s *Service) SyncMacro(ctx context.Context, src port.MacroSource) (Result, error) {
	var r Result
	for _, code := range src.Serves() {
		r.Assets++
		since, err := s.macroWindowStart(ctx, code)
		if err != nil {
			return r, err
		}
		if since.After(s.today()) {
			continue
		}
		points, err := src.Series(ctx, code, since)
		if err != nil {
			return r, fmt.Errorf("%s series %s: %w", src.Name(), code, err)
		}
		if err := s.st.Macro.UpsertMacro(ctx, points); err != nil {
			return r, fmt.Errorf("store series %s: %w", code, err)
		}
		r.Rows += len(points)
	}
	s.opt.Logger.Info("macro done", "source", src.Name(), "series", r.Assets, "rows", r.Rows)
	return r, nil
}

func (s *Service) macroWindowStart(ctx context.Context, code string) (time.Time, error) {
	last, ok, err := s.st.Macro.LatestMacroDate(ctx, code)
	if err != nil {
		return time.Time{}, fmt.Errorf("latest %s: %w", code, err)
	}
	if !ok {
		return s.opt.HistoryStart, nil
	}
	return truncateDay(last).AddDate(0, 0, 1), nil
}

// SyncBonds stores government bond quotes from the day after the newest
// stored one. Each bond is also an asset (class gov_bond), registered here,
// active until it matures; bonds are not part of the stock universe, so
// price, fundamentals and quote routines skip them.
func (s *Service) SyncBonds(ctx context.Context, src port.BondSource) (Result, error) {
	since := s.opt.HistoryStart
	last, ok, err := s.st.Bonds.LatestBondDate(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("latest bond date: %w", err)
	}
	if ok {
		since = truncateDay(last).AddDate(0, 0, 1)
	}
	quotes, err := src.BondQuotes(ctx, since)
	if err != nil {
		return Result{}, fmt.Errorf("bond quotes from %s: %w", src.Name(), err)
	}
	if len(quotes) == 0 {
		return Result{}, nil
	}

	today := s.today()
	seen := map[domain.AssetKey]bool{}
	var bonds []domain.Asset
	for _, q := range quotes {
		if seen[q.Asset] {
			continue
		}
		seen[q.Asset] = true
		bonds = append(bonds, domain.Asset{Key: q.Asset, Class: domain.ClassGovBond, Name: q.Name, Active: !q.Maturity.Before(today)})
	}
	if err := s.st.Assets.UpsertAssets(ctx, bonds); err != nil {
		return Result{}, fmt.Errorf("upsert bonds: %w", err)
	}
	for start := 0; start < len(quotes); start += s.opt.BatchSize {
		if err := s.st.Bonds.UpsertBondQuotes(ctx, quotes[start:min(start+s.opt.BatchSize, len(quotes))]); err != nil {
			return Result{}, fmt.Errorf("store bond quotes: %w", err)
		}
	}
	s.opt.Logger.Info("bonds done", "source", src.Name(), "bonds", len(bonds), "rows", len(quotes))
	return Result{Assets: len(bonds), Rows: len(quotes)}, nil
}
