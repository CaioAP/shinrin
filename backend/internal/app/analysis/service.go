// Package analysis is the read side of the analysis engine. It combines an
// asset's stored scorecard and indicators with macro data and the investor's
// risk profile into a full analysis (composite score, fair values,
// valuation and timing views), ranks assets for the screener, and builds the
// allocation outlook. Everything is recomputed on read from stored data with
// the pure rules in domain/scoring, so a profile change needs no new run.
package analysis

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
	engine "github.com/CaioAP/shinrin/backend/internal/domain/scoring"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Stores are the repositories the service reads.
type Stores struct {
	Assets     port.AssetReader
	Scores     port.ScoreReader
	Indicators port.IndicatorReader
	Macro      port.MacroReader
}

// Service implements port.AnalysisService.
type Service struct {
	st  Stores
	now func() time.Time
}

var _ port.AnalysisService = (*Service)(nil)

// New builds the service; now may be nil.
func New(st Stores, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{st: st, now: now}
}

// riskFreeSeries is each market's nominal risk-free rate for the DCF.
var riskFreeSeries = map[domain.Market]string{
	domain.MarketB3: domain.SeriesSelicTarget,
	domain.MarketUS: domain.SeriesUST10Y,
}

// Analyze implements port.AnalysisService.
func (s *Service) Analyze(ctx context.Context, key domain.AssetKey, p domain.RiskProfile) (domain.Analysis, error) {
	asset, err := s.st.Assets.GetAsset(ctx, key)
	if err != nil {
		return domain.Analysis{}, fmt.Errorf("asset %s: %w", key, err)
	}
	sc, err := s.st.Scores.LatestScorecard(ctx, key)
	if err != nil {
		return domain.Analysis{}, fmt.Errorf("scorecard of %s: %w", key, err)
	}
	a := domain.Analysis{Asset: asset, AsOf: sc.AsOf, Profile: p, Scorecard: sc}
	a.Composite, a.Coverage = engine.Composite(sc, p)

	ind, err := s.st.Indicators.LatestIndicators(ctx, key)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		ind = domain.IndicatorSet{Asset: key, Values: map[string]float64{}}
		a.Notes = append(a.Notes, "No indicators stored, so fair values and timing could not be computed.")
	case err != nil:
		return domain.Analysis{}, fmt.Errorf("indicators of %s: %w", key, err)
	}
	a.Indicators = ind
	metrics := engine.Metrics(ind.Values, nil)
	a.Price = metrics[indicators.Close]

	rf, hasRF, err := s.latest(ctx, riskFreeSeries[key.Market])
	if err != nil {
		return domain.Analysis{}, err
	}
	var notes []string
	a.FairValues, notes = engine.FairValues(key.Market, ind.Values, rf, hasRF)
	a.Notes = append(a.Notes, notes...)
	if m, _ := engine.ModelFor(asset); m.Name == engine.FundModel.Name {
		a.Notes = append(a.Notes, "Quality and growth are not scored for funds until fund reports (vacancy, FFO) are ingested.")
	}
	a.Valuation, a.ValuationSignals = engine.ValuationViewOf(sc, a.Price, a.FairValues)
	a.Timing = engine.TimingOf(metrics, a.Valuation)
	return a, nil
}

// Rank implements port.AnalysisService: assets ordered by composite score
// for the profile, best first.
func (s *Service) Rank(ctx context.Context, f port.RankFilter) ([]port.RankedAsset, error) {
	markets := []domain.Market{domain.MarketB3, domain.MarketUS}
	if f.Market != "" {
		markets = []domain.Market{f.Market}
	}
	var out []port.RankedAsset
	for _, m := range markets {
		assets, err := s.st.Assets.ListAssets(ctx, port.AssetFilter{Market: m, Class: f.Class})
		if err != nil {
			return nil, fmt.Errorf("list %s assets: %w", m, err)
		}
		byKey := make(map[domain.AssetKey]domain.Asset, len(assets))
		for _, a := range assets {
			byKey[a.Key] = a
		}
		cards, err := s.st.Scores.LatestScorecards(ctx, m)
		if err != nil {
			return nil, fmt.Errorf("scorecards of %s: %w", m, err)
		}
		for _, sc := range cards {
			a, ok := byKey[sc.Asset]
			if !ok || !a.Active {
				continue
			}
			c, cov := engine.Composite(sc, f.Profile)
			out = append(out, port.RankedAsset{Asset: a, AsOf: sc.AsOf, Composite: c, Coverage: cov, Factors: sc.Factors})
		}
	}
	slices.SortFunc(out, func(a, b port.RankedAsset) int {
		return cmp.Or(cmp.Compare(b.Composite, a.Composite), cmp.Compare(b.Coverage, a.Coverage), cmp.Compare(a.Asset.Key.String(), b.Asset.Key.String()))
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// Outlook implements port.AnalysisService.
func (s *Service) Outlook(ctx context.Context, p domain.RiskProfile) (domain.Outlook, error) {
	var m engine.MacroContext
	note := func(d time.Time) {
		if d.After(m.AsOf) {
			m.AsOf = d
		}
	}
	for series, dst := range map[string]**float64{domain.SeriesSelicTarget: &m.Selic, domain.SeriesUST10Y: &m.UST10Y, domain.SeriesUST3M: &m.UST3M} {
		pts, err := s.st.Macro.MacroSince(ctx, series, s.now().AddDate(0, -3, 0))
		if err != nil {
			return domain.Outlook{}, fmt.Errorf("macro %s: %w", series, err)
		}
		if len(pts) > 0 {
			v := pts[len(pts)-1].Value
			*dst = &v
			note(pts[len(pts)-1].Date)
		}
	}
	ipca, err := s.st.Macro.MacroSince(ctx, domain.SeriesIPCA, s.now().AddDate(0, -14, 0))
	if err != nil {
		return domain.Outlook{}, fmt.Errorf("macro ipca: %w", err)
	}
	if v, ok := engine.Compound12M(ipca); ok {
		m.IPCA12M = &v
		note(ipca[len(ipca)-1].Date)
	}
	return engine.OutlookFor(p, m), nil
}

// latest returns a series' newest value within the last three months; older
// data is too stale to discount cash flows with.
func (s *Service) latest(ctx context.Context, series string) (float64, bool, error) {
	if series == "" {
		return 0, false, nil
	}
	pts, err := s.st.Macro.MacroSince(ctx, series, s.now().AddDate(0, -3, 0))
	if err != nil {
		return 0, false, fmt.Errorf("macro %s: %w", series, err)
	}
	if len(pts) == 0 {
		return 0, false, nil
	}
	return pts[len(pts)-1].Value, true, nil
}
