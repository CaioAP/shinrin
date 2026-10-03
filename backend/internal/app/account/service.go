// Package account implements port.AccountService: sign-up, sign-in,
// server-side sessions and the risk profile questionnaire.
package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Deps are the driven ports the service needs.
type Deps struct {
	Users    port.UserRepository
	Sessions port.SessionRepository
	Hasher   port.PasswordHasher
}

// Options tune the service; zero values take defaults.
type Options struct {
	// SessionTTL is how long a sign-in lasts (default 30 days).
	SessionTTL time.Duration
	Now        func() time.Time
}

// Service implements port.AccountService.
type Service struct {
	d   Deps
	ttl time.Duration
	now func() time.Time

	dummyOnce sync.Once
	dummy     string

	mu       sync.Mutex
	failures map[string][]time.Time // recent failed sign-ins per email
}

// Sign-in throttling: after maxFailures wrong passwords for one email within
// failureWindow, further attempts are refused until the window passes.
const (
	maxFailures   = 5
	failureWindow = 15 * time.Minute
)

var _ port.AccountService = (*Service)(nil)

// New builds the service.
func New(d Deps, o Options) *Service {
	if o.SessionTTL <= 0 {
		o.SessionTTL = 30 * 24 * time.Hour
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{d: d, ttl: o.SessionTTL, now: o.Now, failures: map[string][]time.Time{}}
}

// SignUp implements port.AccountService.
func (s *Service) SignUp(ctx context.Context, email, password string) (domain.User, port.SessionToken, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return domain.User{}, port.SessionToken{}, err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return domain.User{}, port.SessionToken{}, err
	}
	hash, err := s.d.Hasher.Hash(password)
	if err != nil {
		return domain.User{}, port.SessionToken{}, fmt.Errorf("hash password: %w", err)
	}
	u, err := s.d.Users.CreateUser(ctx, email, hash)
	if err != nil {
		return domain.User{}, port.SessionToken{}, fmt.Errorf("create user: %w", err)
	}
	tok, err := s.newSession(ctx, u.ID)
	return u, tok, err
}

// SignIn implements port.AccountService. Unknown emails still cost one hash
// verification, so response time does not reveal which emails have accounts.
func (s *Service) SignIn(ctx context.Context, email, password string) (domain.User, port.SessionToken, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return domain.User{}, port.SessionToken{}, domain.ErrUnauthorized
	}
	if s.throttled(email) {
		return domain.User{}, port.SessionToken{}, domain.ErrRateLimited
	}
	u, hash, err := s.d.Users.UserByEmail(ctx, email)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		_, _ = s.d.Hasher.Verify(password, s.dummyHash())
		s.recordFailure(email)
		return domain.User{}, port.SessionToken{}, domain.ErrUnauthorized
	case err != nil:
		return domain.User{}, port.SessionToken{}, fmt.Errorf("find user: %w", err)
	}
	ok, err := s.d.Hasher.Verify(password, hash)
	if err != nil {
		return domain.User{}, port.SessionToken{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		s.recordFailure(email)
		return domain.User{}, port.SessionToken{}, domain.ErrUnauthorized
	}
	s.clearFailures(email)
	tok, err := s.newSession(ctx, u.ID)
	return u, tok, err
}

// throttled reports whether email has too many recent failures, dropping
// the ones older than the window.
func (s *Service) throttled(email string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := s.now().Add(-failureWindow)
	recent := s.failures[email][:0]
	for _, t := range s.failures[email] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) == 0 {
		delete(s.failures, email)
		return false
	}
	s.failures[email] = recent
	return len(recent) >= maxFailures
}

// maxTracked bounds the failure map; past it the map is reset, which only
// forgets throttling, never lets a wrong password in.
const maxTracked = 10_000

func (s *Service) recordFailure(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.failures) >= maxTracked {
		s.failures = map[string][]time.Time{}
	}
	s.failures[email] = append(s.failures[email], s.now())
}

func (s *Service) clearFailures(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, email)
}

// SignOut implements port.AccountService.
func (s *Service) SignOut(ctx context.Context, token string) error {
	if err := s.d.Sessions.DeleteSession(ctx, hashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Authenticate implements port.AccountService.
func (s *Service) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if token == "" {
		return domain.User{}, domain.ErrUnauthorized
	}
	h := hashToken(token)
	sess, err := s.d.Sessions.SessionByTokenHash(ctx, h)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return domain.User{}, domain.ErrUnauthorized
	case err != nil:
		return domain.User{}, fmt.Errorf("find session: %w", err)
	}
	if !s.now().Before(sess.ExpiresAt) {
		_ = s.d.Sessions.DeleteSession(ctx, h)
		return domain.User{}, domain.ErrUnauthorized
	}
	u, err := s.d.Users.UserByID(ctx, sess.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, domain.ErrUnauthorized
	}
	return u, err
}

// SetRiskProfile implements port.AccountService.
func (s *Service) SetRiskProfile(ctx context.Context, id domain.UserID, answers domain.SuitabilityAnswers) (domain.User, error) {
	p, _, err := domain.ProfileFromAnswers(answers)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.d.Users.SaveRiskProfile(ctx, id, p, answers, s.now()); err != nil {
		return domain.User{}, fmt.Errorf("save risk profile: %w", err)
	}
	return s.d.Users.UserByID(ctx, id)
}

// DeleteAccount implements port.AccountService.
func (s *Service) DeleteAccount(ctx context.Context, id domain.UserID, password string) error {
	hash, err := s.d.Users.PasswordHash(ctx, id)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}
	ok, err := s.d.Hasher.Verify(password, hash)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return domain.ErrUnauthorized
	}
	if err := s.d.Users.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (s *Service) newSession(ctx context.Context, id domain.UserID) (port.SessionToken, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return port.SessionToken{}, fmt.Errorf("session token: %w", err)
	}
	tok := base64.RawURLEncoding.EncodeToString(b[:])
	now := s.now()
	sess := domain.Session{TokenHash: hashToken(tok), UserID: id, CreatedAt: now, ExpiresAt: now.Add(s.ttl)}
	if err := s.d.Sessions.CreateSession(ctx, sess); err != nil {
		return port.SessionToken{}, fmt.Errorf("create session: %w", err)
	}
	return port.SessionToken{Value: tok, ExpiresAt: sess.ExpiresAt}, nil
}

// dummyHash is a real hash of nothing in particular, verified against when
// an email is unknown.
func (s *Service) dummyHash() string {
	s.dummyOnce.Do(func() { s.dummy, _ = s.d.Hasher.Hash("shinrin-timing-equaliser") })
	return s.dummy
}

// hashToken is SHA-256: tokens are 256 random bits, so a fast hash is enough
// (no dictionary to attack, unlike passwords).
func hashToken(tok string) []byte {
	h := sha256.Sum256([]byte(tok))
	return h[:]
}
