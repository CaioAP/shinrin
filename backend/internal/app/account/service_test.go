package account_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/account"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// plainHasher keeps tests fast; argon2 has its own tests.
type plainHasher struct{}

func (plainHasher) Hash(pw string) (string, error)       { return "h:" + pw, nil }
func (plainHasher) Verify(pw, hash string) (bool, error) { return hash == "h:"+pw, nil }

var ctx = context.Background()

const pw = "correct horse battery"

func newService(now *time.Time) (*account.Service, *memory.AccountStore) {
	st := memory.NewAccountStore()
	return account.New(account.Deps{Users: st, Sessions: st, Hasher: plainHasher{}}, account.Options{
		SessionTTL: time.Hour, Now: func() time.Time { return *now },
	}), st
}

func TestSignUpSignInAndSessions(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	svc, _ := newService(&now)

	u, tok, err := svc.SignUp(ctx, " Caio@Example.com ", pw)
	if err != nil || u.Email != "caio@example.com" || tok.Value == "" || !tok.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("SignUp = %+v, %+v, %v", u, tok, err)
	}
	if _, _, err := svc.SignUp(ctx, "caio@example.com", pw); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate sign-up err = %v", err)
	}
	if got, err := svc.Authenticate(ctx, tok.Value); err != nil || got.ID != u.ID {
		t.Errorf("Authenticate = %+v, %v", got, err)
	}

	if _, _, err := svc.SignIn(ctx, "caio@example.com", "wrong password!"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("wrong password err = %v", err)
	}
	if _, _, err := svc.SignIn(ctx, "nobody@example.com", pw); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("unknown email err = %v", err)
	}
	_, tok2, err := svc.SignIn(ctx, "CAIO@example.com", pw)
	if err != nil || tok2.Value == tok.Value {
		t.Fatalf("SignIn = %+v, %v", tok2, err)
	}

	if err := svc.SignOut(ctx, tok.Value); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, tok.Value); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("signed-out token err = %v", err)
	}
	now = now.Add(2 * time.Hour)
	if _, err := svc.Authenticate(ctx, tok2.Value); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expired token err = %v", err)
	}
	if _, err := svc.Authenticate(ctx, ""); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("empty token err = %v", err)
	}
}

func TestSignInThrottlesRepeatedFailures(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	svc, _ := newService(&now)
	if _, _, err := svc.SignUp(ctx, "caio@example.com", pw); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		_, _, _ = svc.SignIn(ctx, "caio@example.com", "wrong password!")
	}
	if _, _, err := svc.SignIn(ctx, "caio@example.com", pw); !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("after 5 failures err = %v, want rate limited even with the right password", err)
	}
	now = now.Add(16 * time.Minute)
	if _, _, err := svc.SignIn(ctx, "caio@example.com", pw); err != nil {
		t.Fatalf("after the window err = %v", err)
	}
}

func TestRiskProfileAndDeletion(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	svc, st := newService(&now)
	u, tok, _ := svc.SignUp(ctx, "caio@example.com", pw)

	answers := domain.SuitabilityAnswers{}
	for _, q := range domain.SuitabilityQuestions {
		answers[q.ID] = q.Answers[3]
	}
	got, err := svc.SetRiskProfile(ctx, u.ID, answers)
	if err != nil || got.Profile != domain.ProfileAggressive || !got.ProfileAt.Equal(now) || got.ProfileAnswers["horizon"] != "gt_5y" {
		t.Fatalf("SetRiskProfile = %+v, %v", got, err)
	}
	if _, err := svc.SetRiskProfile(ctx, u.ID, domain.SuitabilityAnswers{"horizon": "gt_5y"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("incomplete answers err = %v", err)
	}

	if err := svc.DeleteAccount(ctx, u.ID, "wrong password!"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("delete with wrong password err = %v", err)
	}
	if err := svc.DeleteAccount(ctx, u.ID, pw); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByID(ctx, u.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("user still stored: %v", err)
	}
	if _, err := svc.Authenticate(ctx, tok.Value); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("session survived deletion: %v", err)
	}
}
