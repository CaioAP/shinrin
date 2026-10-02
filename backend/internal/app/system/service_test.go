package system_test

import (
	"context"
	"errors"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/app/system"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

type stubChecker struct {
	name string
	err  error
}

func (s stubChecker) Name() string                { return s.name }
func (s stubChecker) Check(context.Context) error { return s.err }

func TestInfoCarriesDisclaimer(t *testing.T) {
	info := system.New("test").Info(context.Background())
	if info.Disclaimer != domain.Disclaimer {
		t.Fatalf("disclaimer = %q", info.Disclaimer)
	}
}

func TestHealthFailsWhenAnyCheckerFails(t *testing.T) {
	svc := system.New("test", stubChecker{name: "db"}, stubChecker{name: "cache", err: errors.New("down")})
	r := svc.Health(context.Background())
	if r.OK {
		t.Fatal("report OK, want failure")
	}
	if r.Checks["db"] != "ok" || r.Checks["cache"] != "down" {
		t.Fatalf("checks = %v", r.Checks)
	}
}
