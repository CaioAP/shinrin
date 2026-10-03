package userreport_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/userreport"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

var (
	now  = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	petr = domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}
)

type creds struct {
	cap int
	err error
}

func (c creds) CredentialFor(context.Context, domain.UserID) (port.LLMCredential, domain.LLMSettings, error) {
	return port.LLMCredential{Provider: "anthropic", APIKey: "sk-1234567890"}, domain.LLMSettings{MonthlyCap: c.cap}, c.err
}
func (c creds) Settings(context.Context, domain.UserID) (domain.LLMSettings, error) {
	return domain.LLMSettings{MonthlyCap: c.cap}, c.err
}

// writer stores a report like app/report would, optionally blocking.
type writer struct {
	store   *memory.AnalysisStore
	gate    chan struct{}
	started chan struct{}
	last    port.ReportRequest
}

func (w *writer) Generate(ctx context.Context, req port.ReportRequest) (domain.Report, error) {
	w.last = req
	if w.gate != nil {
		close(w.started)
		<-w.gate
	}
	r := domain.Report{UserID: req.User, Asset: req.Asset, Profile: req.Profile, CreatedAt: now}
	var err error
	r.ID, err = w.store.SaveReport(ctx, r)
	return r, err
}

func setup(c creds) (*userreport.Service, *writer, *memory.AnalysisStore) {
	store := memory.NewAnalysisStore()
	w := &writer{store: store}
	return userreport.New(userreport.Deps{Credentials: c, Reports: w, Library: store}, func() time.Time { return now }), w, store
}

func TestGenerateUsesOwnerAndCap(t *testing.T) {
	svc, w, _ := setup(creds{cap: 2})
	ctx := context.Background()
	for range 2 {
		if _, err := svc.Generate(ctx, 7, petr, "Aggressive", "fr"); err != nil {
			t.Fatal(err)
		}
	}
	if w.last.User != 7 || w.last.Profile != domain.ProfileAggressive || w.last.Lang != "en" || w.last.Credential.APIKey == "" {
		t.Fatalf("request = %+v", w.last)
	}
	if _, err := svc.Generate(ctx, 7, petr, domain.ProfileModerate, "pt-BR"); !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("over cap err = %v", err)
	}
	u, err := svc.Usage(ctx, 7)
	if err != nil || u.Used != 2 || u.Cap != 2 || !u.Since.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("Usage = %+v, %v", u, err)
	}
	// Another user has their own count and cannot read user 7's reports.
	if _, err := svc.Generate(ctx, 8, petr, domain.ProfileModerate, "en"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, 8, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get other's report = %v", err)
	}
	list, _ := svc.List(ctx, 7, petr, 0)
	if len(list) != 2 || list[0].ID != 2 {
		t.Fatalf("List = %+v", list)
	}
}

func TestGenerateOneAtATime(t *testing.T) {
	svc, w, _ := setup(creds{cap: 10})
	w.gate, w.started = make(chan struct{}), make(chan struct{})
	done := make(chan error)
	go func() {
		_, err := svc.Generate(context.Background(), 7, petr, domain.ProfileModerate, "en")
		done <- err
	}()
	<-w.started
	if _, err := svc.Generate(context.Background(), 7, petr, domain.ProfileModerate, "en"); !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("concurrent err = %v", err)
	}
	close(w.gate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestGenerateNeedsKey(t *testing.T) {
	svc, _, _ := setup(creds{err: domain.ErrNotFound})
	if _, err := svc.Generate(context.Background(), 7, petr, domain.ProfileModerate, "en"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	u, err := svc.Usage(context.Background(), 7)
	if err != nil || u.Cap != domain.DefaultMonthlyReportCap {
		t.Fatalf("Usage = %+v, %v", u, err)
	}
}
