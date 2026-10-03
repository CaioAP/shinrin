// Package market implements port.MarketService: read-only views of stored
// prices, distributions, news and macro series for the web app.
package market

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	engine "github.com/CaioAP/shinrin/backend/internal/domain/scoring"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Stores are the repositories the service reads.
type Stores struct {
	Assets  port.AssetReader
	Prices  port.PriceReader
	Actions port.CorporateActionReader
	News    port.NewsReader
	Macro   port.MacroReader
}

// Service implements port.MarketService.
type Service struct {
	st  Stores
	now func() time.Time
}

var _ port.MarketService = (*Service)(nil)

// New builds the service; now may be nil.
func New(st Stores, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{st: st, now: now}
}

// newsWindow is how far back News looks.
const newsWindow = 90 * 24 * time.Hour

// PriceHistory implements port.MarketService.
func (s *Service) PriceHistory(ctx context.Context, key domain.AssetKey, from time.Time) ([]domain.PriceBar, error) {
	if _, err := s.st.Assets.GetAsset(ctx, key); err != nil {
		return nil, fmt.Errorf("asset %s: %w", key, err)
	}
	bars, err := s.st.Prices.PricesBetween(ctx, key, from, s.now())
	if err != nil {
		return nil, fmt.Errorf("prices of %s: %w", key, err)
	}
	// Only actions inside the window move bars inside the window.
	actions, err := s.st.Actions.CorporateActionsSince(ctx, key, from)
	if err != nil {
		return nil, fmt.Errorf("corporate actions of %s: %w", key, err)
	}
	return domain.AdjustCloses(bars, actions), nil
}

// Dividends implements port.MarketService.
func (s *Service) Dividends(ctx context.Context, key domain.AssetKey, since time.Time) ([]domain.CorporateAction, error) {
	if _, err := s.st.Assets.GetAsset(ctx, key); err != nil {
		return nil, fmt.Errorf("asset %s: %w", key, err)
	}
	all, err := s.st.Actions.CorporateActionsSince(ctx, key, since)
	if err != nil {
		return nil, fmt.Errorf("corporate actions of %s: %w", key, err)
	}
	var out []domain.CorporateAction
	for _, a := range all {
		if a.IsCash() {
			out = append(out, a)
		}
	}
	slices.SortFunc(out, func(a, b domain.CorporateAction) int { return b.ExDate.Compare(a.ExDate) })
	return out, nil
}

// News implements port.MarketService.
func (s *Service) News(ctx context.Context, key domain.AssetKey, limit int) ([]domain.NewsItem, error) {
	if _, err := s.st.Assets.GetAsset(ctx, key); err != nil {
		return nil, fmt.Errorf("asset %s: %w", key, err)
	}
	items, err := s.st.News.NewsFor(ctx, key, s.now().Add(-newsWindow), limit)
	if err != nil {
		return nil, fmt.Errorf("news of %s: %w", key, err)
	}
	return items, nil
}

// macroLookbackMonths covers fourteen months, enough for twelve-month IPCA and
// year-over-year CPI.
const macroLookbackMonths = 14

// Macro implements port.MarketService.
func (s *Service) Macro(ctx context.Context) (domain.MacroStrip, error) {
	since := s.now().AddDate(0, -macroLookbackMonths, 0)
	codes := []string{
		domain.SeriesSelicTarget, domain.SeriesCDI, domain.SeriesIPCA, domain.SeriesUSDBRL,
		domain.SeriesFedFunds, domain.SeriesUST10Y, domain.SeriesUSCPI,
	}
	series := make(map[string][]domain.MacroPoint, len(codes))
	for _, c := range codes {
		pts, err := s.st.Macro.MacroSince(ctx, c, since)
		if err != nil {
			return domain.MacroStrip{}, fmt.Errorf("macro %s: %w", c, err)
		}
		series[c] = pts
	}
	return engine.MacroStripOf(series), nil
}
