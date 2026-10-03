// Package userreport implements port.UserReportService: AI reports for
// signed-in users, run with their saved key, inside the monthly cap they
// chose, one at a time per user, and readable only by their owner.
package userreport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Deps are the ports the service needs.
type Deps struct {
	Credentials port.CredentialSource
	Reports     port.ReportService
	Library     port.ReportReader
}

// Service implements port.UserReportService.
type Service struct {
	d   Deps
	now func() time.Time

	mu       sync.Mutex
	inFlight map[domain.UserID]bool
}

var _ port.UserReportService = (*Service)(nil)

// MaxList bounds how many reports List returns.
const MaxList = 20

// New builds the service; now may be nil (time.Now).
func New(d Deps, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{d: d, now: now, inFlight: map[domain.UserID]bool{}}
}

// Generate implements port.UserReportService.
func (s *Service) Generate(ctx context.Context, user domain.UserID, asset domain.AssetKey, profile domain.RiskProfile, lang string) (domain.Report, error) {
	profile, err := domain.ParseRiskProfile(string(profile))
	if err != nil {
		return domain.Report{}, err
	}
	if lang != "pt-BR" {
		lang = "en"
	}
	if !s.acquire(user) {
		return domain.Report{}, fmt.Errorf("%w: a report of yours is already being written", domain.ErrRateLimited)
	}
	defer s.release(user)

	cred, set, err := s.d.Credentials.CredentialFor(ctx, user)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Report{}, fmt.Errorf("%w: save your LLM API key in Settings first", domain.ErrInvalid)
	}
	if err != nil {
		return domain.Report{}, err
	}
	used, err := s.d.Library.CountReportsSince(ctx, user, domain.MonthStart(s.now()))
	if err != nil {
		return domain.Report{}, err
	}
	if used >= set.MonthlyCap {
		return domain.Report{}, fmt.Errorf("%w: you reached your monthly cap of %d reports; raise it in Settings", domain.ErrRateLimited, set.MonthlyCap)
	}
	return s.d.Reports.Generate(ctx, port.ReportRequest{User: user, Asset: asset, Profile: profile, Lang: lang, Credential: cred})
}

func (s *Service) acquire(user domain.UserID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight[user] {
		return false
	}
	s.inFlight[user] = true
	return true
}

func (s *Service) release(user domain.UserID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inFlight, user)
}

// List implements port.UserReportService.
func (s *Service) List(ctx context.Context, user domain.UserID, asset domain.AssetKey, limit int) ([]domain.Report, error) {
	if limit <= 0 || limit > MaxList {
		limit = MaxList
	}
	return s.d.Library.ReportsFor(ctx, user, asset, limit)
}

// Get implements port.UserReportService.
func (s *Service) Get(ctx context.Context, user domain.UserID, id int64) (domain.Report, error) {
	return s.d.Library.Report(ctx, user, id)
}

// Usage implements port.UserReportService. Without a saved key (or with
// saved keys switched off) the cap is the default one.
func (s *Service) Usage(ctx context.Context, user domain.UserID) (domain.LLMUsage, error) {
	u := domain.LLMUsage{Cap: domain.DefaultMonthlyReportCap, Since: domain.MonthStart(s.now())}
	if set, err := s.d.Credentials.Settings(ctx, user); err == nil {
		u.Cap = set.MonthlyCap
	} else if !errors.Is(err, domain.ErrNotFound) && !errors.Is(err, domain.ErrUnavailable) {
		return domain.LLMUsage{}, err
	}
	var err error
	u.Used, err = s.d.Library.CountReportsSince(ctx, user, u.Since)
	return u, err
}
