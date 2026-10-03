package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// MarketDataStore keeps prices, fundamentals, corporate actions and
// indicators in memory. It mirrors the Postgres adapter's upsert keys so
// service tests behave the same against either.
type MarketDataStore struct {
	mu           sync.RWMutex
	prices       map[domain.AssetKey]map[time.Time]domain.PriceBar
	fundamentals map[domain.AssetKey]map[fundamentalKey]domain.Fundamental
	actions      map[domain.AssetKey]map[actionKey]domain.CorporateAction
	indicators   map[domain.AssetKey]domain.IndicatorSet // latest only
}

type fundamentalKey struct {
	end    time.Time
	period domain.PeriodType
	metric string
}

type actionKey struct {
	exDate time.Time
	typ    domain.CorporateActionType
}

var (
	_ port.PriceRepository           = (*MarketDataStore)(nil)
	_ port.FundamentalRepository     = (*MarketDataStore)(nil)
	_ port.CorporateActionRepository = (*MarketDataStore)(nil)
	_ port.IndicatorReader           = (*MarketDataStore)(nil)
	_ port.IndicatorWriter           = (*MarketDataStore)(nil)
)

// NewMarketDataStore returns an empty store.
func NewMarketDataStore() *MarketDataStore {
	return &MarketDataStore{
		prices:       make(map[domain.AssetKey]map[time.Time]domain.PriceBar),
		fundamentals: make(map[domain.AssetKey]map[fundamentalKey]domain.Fundamental),
		actions:      make(map[domain.AssetKey]map[actionKey]domain.CorporateAction),
		indicators:   make(map[domain.AssetKey]domain.IndicatorSet),
	}
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// UpsertPrices implements port.PriceWriter.
func (s *MarketDataStore) UpsertPrices(_ context.Context, bars []domain.PriceBar) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range bars {
		b.Date = dateOnly(b.Date)
		if s.prices[b.Asset] == nil {
			s.prices[b.Asset] = make(map[time.Time]domain.PriceBar)
		}
		s.prices[b.Asset][b.Date] = b
	}
	return nil
}

// PricesBetween implements port.PriceReader.
func (s *MarketDataStore) PricesBetween(_ context.Context, asset domain.AssetKey, from, to time.Time) ([]domain.PriceBar, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	from, to = dateOnly(from), dateOnly(to)
	var out []domain.PriceBar
	for d, b := range s.prices[asset] {
		if !d.Before(from) && !d.After(to) {
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b domain.PriceBar) int { return a.Date.Compare(b.Date) })
	return out, nil
}

// LatestPriceDate implements port.PriceReader.
func (s *MarketDataStore) LatestPriceDate(_ context.Context, asset domain.AssetKey) (time.Time, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest time.Time
	for d := range s.prices[asset] {
		if d.After(latest) {
			latest = d
		}
	}
	return latest, !latest.IsZero(), nil
}

// UpsertFundamentals implements port.FundamentalWriter.
func (s *MarketDataStore) UpsertFundamentals(_ context.Context, rows []domain.Fundamental) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		r.PeriodEnd = dateOnly(r.PeriodEnd)
		if s.fundamentals[r.Asset] == nil {
			s.fundamentals[r.Asset] = make(map[fundamentalKey]domain.Fundamental)
		}
		s.fundamentals[r.Asset][fundamentalKey{r.PeriodEnd, r.PeriodType, r.Metric}] = r
	}
	return nil
}

// FundamentalsSince implements port.FundamentalReader.
func (s *MarketDataStore) FundamentalsSince(_ context.Context, asset domain.AssetKey, since time.Time) ([]domain.Fundamental, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Fundamental
	for _, r := range s.fundamentals[asset] {
		if !r.PeriodEnd.Before(dateOnly(since)) {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b domain.Fundamental) int {
		return cmp.Or(a.PeriodEnd.Compare(b.PeriodEnd), cmp.Compare(a.Metric, b.Metric), cmp.Compare(a.PeriodType, b.PeriodType))
	})
	return out, nil
}

// LatestPeriodEnd implements port.FundamentalReader.
func (s *MarketDataStore) LatestPeriodEnd(_ context.Context, asset domain.AssetKey) (time.Time, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest time.Time
	for k := range s.fundamentals[asset] {
		if k.end.After(latest) {
			latest = k.end
		}
	}
	return latest, !latest.IsZero(), nil
}

// UpsertCorporateActions implements port.CorporateActionWriter.
func (s *MarketDataStore) UpsertCorporateActions(_ context.Context, actions []domain.CorporateAction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range actions {
		a.ExDate = dateOnly(a.ExDate)
		if s.actions[a.Asset] == nil {
			s.actions[a.Asset] = make(map[actionKey]domain.CorporateAction)
		}
		s.actions[a.Asset][actionKey{a.ExDate, a.Type}] = a
	}
	return nil
}

// CorporateActionsSince implements port.CorporateActionReader.
func (s *MarketDataStore) CorporateActionsSince(_ context.Context, asset domain.AssetKey, since time.Time) ([]domain.CorporateAction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.CorporateAction
	for _, a := range s.actions[asset] {
		if !a.ExDate.Before(dateOnly(since)) {
			out = append(out, a)
		}
	}
	slices.SortFunc(out, func(a, b domain.CorporateAction) int {
		return cmp.Or(a.ExDate.Compare(b.ExDate), cmp.Compare(a.Type, b.Type))
	})
	return out, nil
}

// UpsertIndicators implements port.IndicatorWriter. Only the newest set per
// asset is kept.
func (s *MarketDataStore) UpsertIndicators(_ context.Context, sets []domain.IndicatorSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, set := range sets {
		if cur, ok := s.indicators[set.Asset]; ok && cur.AsOf.After(set.AsOf) {
			continue
		}
		set.AsOf = dateOnly(set.AsOf)
		s.indicators[set.Asset] = set
	}
	return nil
}

// LatestIndicators implements port.IndicatorReader.
func (s *MarketDataStore) LatestIndicators(_ context.Context, asset domain.AssetKey) (domain.IndicatorSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, ok := s.indicators[asset]
	if !ok {
		return domain.IndicatorSet{}, domain.ErrNotFound
	}
	return set, nil
}
