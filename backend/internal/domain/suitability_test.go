package domain_test

import (
	"errors"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func answers(idx int, override map[string]string) domain.SuitabilityAnswers {
	a := domain.SuitabilityAnswers{}
	for _, q := range domain.SuitabilityQuestions {
		a[q.ID] = q.Answers[idx]
	}
	for k, v := range override {
		a[k] = v
	}
	return a
}

func TestProfileFromAnswers(t *testing.T) {
	tests := []struct {
		name string
		in   domain.SuitabilityAnswers
		want domain.RiskProfile
	}{
		{"all cautious", answers(0, nil), domain.ProfileConservative},
		{"all second", answers(1, nil), domain.ProfileConservative},
		{"all third", answers(2, nil), domain.ProfileModerate},
		{"all boldest", answers(3, nil), domain.ProfileAggressive},
		{"bold but money needed this year", answers(3, map[string]string{"horizon": "lt_1y"}), domain.ProfileConservative},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := domain.ProfileFromAnswers(tt.in)
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestProfileFromAnswersRejectsIncomplete(t *testing.T) {
	a := answers(1, nil)
	delete(a, "liquidity")
	if _, _, err := domain.ProfileFromAnswers(a); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("missing answer: err = %v", err)
	}
	a["liquidity"] = "lots"
	if _, _, err := domain.ProfileFromAnswers(a); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("unknown answer: err = %v", err)
	}
}

func TestNormalizeEmailAndPassword(t *testing.T) {
	if e, err := domain.NormalizeEmail("  Caio@Example.COM "); err != nil || e != "caio@example.com" {
		t.Errorf("NormalizeEmail = %q, %v", e, err)
	}
	for _, bad := range []string{"", "nope", "Caio <caio@example.com>", "a@"} {
		if _, err := domain.NormalizeEmail(bad); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("NormalizeEmail(%q) err = %v", bad, err)
		}
	}
	if err := domain.ValidatePassword("short"); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("short password accepted")
	}
	if err := domain.ValidatePassword("correct horse battery"); err != nil {
		t.Errorf("good password rejected: %v", err)
	}
}
