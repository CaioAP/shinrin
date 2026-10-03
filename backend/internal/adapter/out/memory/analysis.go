package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// AnalysisStore keeps scorecards (latest per asset) and AI reports in
// memory.
type AnalysisStore struct {
	mu      sync.RWMutex
	scores  map[domain.AssetKey]domain.Scorecard
	reports []domain.Report
}

var (
	_ port.ScoreReader  = (*AnalysisStore)(nil)
	_ port.ScoreWriter  = (*AnalysisStore)(nil)
	_ port.ReportWriter = (*AnalysisStore)(nil)
)

// NewAnalysisStore returns an empty store.
func NewAnalysisStore() *AnalysisStore {
	return &AnalysisStore{scores: map[domain.AssetKey]domain.Scorecard{}}
}

// UpsertScorecards implements port.ScoreWriter. An older scorecard never
// replaces a newer one.
func (s *AnalysisStore) UpsertScorecards(_ context.Context, cards []domain.Scorecard) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range cards {
		if cur, ok := s.scores[c.Asset]; ok && cur.AsOf.After(c.AsOf) {
			continue
		}
		c.AsOf = dateOnly(c.AsOf)
		s.scores[c.Asset] = c
	}
	return nil
}

// LatestScorecard implements port.ScoreReader.
func (s *AnalysisStore) LatestScorecard(_ context.Context, asset domain.AssetKey) (domain.Scorecard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.scores[asset]
	if !ok {
		return domain.Scorecard{}, domain.ErrNotFound
	}
	return c, nil
}

// LatestScorecards implements port.ScoreReader.
func (s *AnalysisStore) LatestScorecards(_ context.Context, market domain.Market) ([]domain.Scorecard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Scorecard
	for k, c := range s.scores {
		if k.Market == market {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b domain.Scorecard) int { return cmp.Compare(a.Asset.String(), b.Asset.String()) })
	return out, nil
}

// SaveReport implements port.ReportWriter.
func (s *AnalysisStore) SaveReport(_ context.Context, r domain.Report) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = int64(len(s.reports) + 1)
	s.reports = append(s.reports, r)
	return r.ID, nil
}

// Reports returns the stored reports, oldest first (for tests and the
// in-memory api role).
func (s *AnalysisStore) Reports() []domain.Report {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.reports)
}
