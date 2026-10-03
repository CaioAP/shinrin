package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// FeedStore keeps quotes, news, macro series and bond quotes in memory,
// mirroring the Postgres adapter's keys.
type FeedStore struct {
	mu     sync.RWMutex
	quotes map[domain.AssetKey]domain.Quote
	news   map[string]domain.NewsItem // by URL
	macro  map[string]map[time.Time]domain.MacroPoint
	bonds  map[domain.AssetKey]map[time.Time]domain.BondQuote
}

var (
	_ port.QuoteWriter     = (*FeedStore)(nil)
	_ port.QuoteReader     = (*FeedStore)(nil)
	_ port.NewsWriter      = (*FeedStore)(nil)
	_ port.NewsReader      = (*FeedStore)(nil)
	_ port.MacroRepository = (*FeedStore)(nil)
	_ port.BondRepository  = (*FeedStore)(nil)
)

// NewFeedStore returns an empty store.
func NewFeedStore() *FeedStore {
	return &FeedStore{
		quotes: map[domain.AssetKey]domain.Quote{},
		news:   map[string]domain.NewsItem{},
		macro:  map[string]map[time.Time]domain.MacroPoint{},
		bonds:  map[domain.AssetKey]map[time.Time]domain.BondQuote{},
	}
}

// UpsertQuotes implements port.QuoteWriter; an older quote never replaces a
// newer one.
func (s *FeedStore) UpsertQuotes(_ context.Context, quotes []domain.Quote) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range quotes {
		if cur, ok := s.quotes[q.Asset]; ok && cur.AsOf.After(q.AsOf) {
			continue
		}
		s.quotes[q.Asset] = q
	}
	return nil
}

// LatestQuote implements port.QuoteReader.
func (s *FeedStore) LatestQuote(_ context.Context, asset domain.AssetKey) (domain.Quote, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q, ok := s.quotes[asset]
	if !ok {
		return domain.Quote{}, domain.ErrNotFound
	}
	return q, nil
}

// UpsertNews implements port.NewsWriter; a repeated URL merges its assets.
func (s *FeedStore) UpsertNews(_ context.Context, items []domain.NewsItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range items {
		if cur, ok := s.news[it.URL]; ok {
			for _, a := range cur.Assets {
				if !slices.Contains(it.Assets, a) {
					it.Assets = append(it.Assets, a)
				}
			}
			it.PublishedAt, it.Source = cur.PublishedAt, cur.Source
		}
		s.news[it.URL] = it
	}
	return nil
}

// NewsFor implements port.NewsReader.
func (s *FeedStore) NewsFor(_ context.Context, asset domain.AssetKey, since time.Time, limit int) ([]domain.NewsItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.NewsItem
	for _, it := range s.news {
		if slices.Contains(it.Assets, asset) && !it.PublishedAt.Before(since) {
			out = append(out, it)
		}
	}
	slices.SortFunc(out, func(a, b domain.NewsItem) int { return b.PublishedAt.Compare(a.PublishedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// UpsertMacro implements port.MacroRepository.
func (s *FeedStore) UpsertMacro(_ context.Context, points []domain.MacroPoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range points {
		p.Date = dateOnly(p.Date)
		if s.macro[p.Series] == nil {
			s.macro[p.Series] = map[time.Time]domain.MacroPoint{}
		}
		s.macro[p.Series][p.Date] = p
	}
	return nil
}

// MacroSince implements port.MacroRepository.
func (s *FeedStore) MacroSince(_ context.Context, series string, since time.Time) ([]domain.MacroPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.MacroPoint
	for d, p := range s.macro[series] {
		if !d.Before(dateOnly(since)) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b domain.MacroPoint) int { return a.Date.Compare(b.Date) })
	return out, nil
}

// LatestMacroDate implements port.MacroRepository.
func (s *FeedStore) LatestMacroDate(_ context.Context, series string) (time.Time, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest time.Time
	for d := range s.macro[series] {
		if d.After(latest) {
			latest = d
		}
	}
	return latest, !latest.IsZero(), nil
}

// UpsertBondQuotes implements port.BondRepository.
func (s *FeedStore) UpsertBondQuotes(_ context.Context, quotes []domain.BondQuote) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range quotes {
		q.Date = dateOnly(q.Date)
		if s.bonds[q.Asset] == nil {
			s.bonds[q.Asset] = map[time.Time]domain.BondQuote{}
		}
		s.bonds[q.Asset][q.Date] = q
	}
	return nil
}

// LatestBondDate implements port.BondRepository.
func (s *FeedStore) LatestBondDate(context.Context) (time.Time, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest time.Time
	for _, byDate := range s.bonds {
		for d := range byDate {
			if d.After(latest) {
				latest = d
			}
		}
	}
	return latest, !latest.IsZero(), nil
}
