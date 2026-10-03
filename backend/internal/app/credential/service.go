// Package credential manages users' own LLM accounts (bring your own key).
// Keys are sealed with a port.SecretBox bound to the owner before they are
// stored, are write-only through port.CredentialService, and are opened
// only for one call through port.CredentialSource.
package credential

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Deps are the ports the service needs. A nil Box switches saved keys off
// (the server has no master key): every call returns domain.ErrUnavailable.
type Deps struct {
	Store port.CredentialRepository
	Box   port.SecretBox
	LLM   port.LLMConnector
}

// Service implements port.CredentialService and port.CredentialSource.
type Service struct {
	d   Deps
	now func() time.Time
}

var (
	_ port.CredentialService = (*Service)(nil)
	_ port.CredentialSource  = (*Service)(nil)
)

// New builds the service; now may be nil (time.Now).
func New(d Deps, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{d: d, now: now}
}

var errDisabled = fmt.Errorf("%w: saved LLM keys need SHINRIN_MASTER_KEY on the server", domain.ErrUnavailable)

// aad binds a sealed key to its owner.
func aad(user domain.UserID) []byte {
	return []byte("shinrin/llm-key/v1/user:" + strconv.FormatInt(int64(user), 10))
}

// Settings implements port.CredentialService.
func (s *Service) Settings(ctx context.Context, user domain.UserID) (domain.LLMSettings, error) {
	if s.d.Box == nil {
		return domain.LLMSettings{}, errDisabled
	}
	c, err := s.d.Store.Credential(ctx, user)
	return c.Settings, err
}

// Save implements port.CredentialService.
func (s *Service) Save(ctx context.Context, user domain.UserID, in port.CredentialInput) (domain.LLMSettings, error) {
	if s.d.Box == nil {
		return domain.LLMSettings{}, errDisabled
	}
	set := domain.LLMSettings{
		Provider:  strings.ToLower(strings.TrimSpace(in.Provider)),
		Model:     strings.TrimSpace(in.Model),
		UpdatedAt: s.now(),
	}
	var err error
	if set.BaseURL, err = domain.ValidateLLMEndpoint(in.BaseURL); err != nil {
		return domain.LLMSettings{}, err
	}
	if set.MonthlyCap, err = domain.NormalizeMonthlyCap(in.MonthlyCap); err != nil {
		return domain.LLMSettings{}, err
	}
	if len(set.Model) > 100 {
		return domain.LLMSettings{}, fmt.Errorf("%w: model name is too long", domain.ErrInvalid)
	}

	key := strings.TrimSpace(in.APIKey)
	var sealed []byte
	if key == "" {
		// Keep the saved key; only a key for the same provider and
		// endpoint may be kept, so it is never sent somewhere new.
		cur, err := s.d.Store.Credential(ctx, user)
		if errors.Is(err, domain.ErrNotFound) {
			return domain.LLMSettings{}, fmt.Errorf("%w: an API key is required", domain.ErrInvalid)
		}
		if err != nil {
			return domain.LLMSettings{}, err
		}
		if cur.Settings.Provider != set.Provider || cur.Settings.BaseURL != set.BaseURL {
			return domain.LLMSettings{}, fmt.Errorf("%w: enter the API key again to change the provider or base URL", domain.ErrInvalid)
		}
		if key, err = s.open(cur, user); err != nil {
			return domain.LLMSettings{}, err
		}
		sealed, set.KeyHint = cur.SealedKey, cur.Settings.KeyHint
	} else {
		if err := validKey(key); err != nil {
			return domain.LLMSettings{}, err
		}
		set.KeyHint = port.KeyHint(key)
	}
	// The connector knows the providers and what each requires (an
	// OpenAI-compatible endpoint needs a model name).
	if _, err := s.d.LLM.Connect(port.LLMCredential{Provider: set.Provider, Model: set.Model, APIKey: key, BaseURL: set.BaseURL}); err != nil {
		return domain.LLMSettings{}, port.RedactKey(err, key)
	}
	if sealed == nil {
		if sealed, err = s.d.Box.Seal([]byte(key), aad(user)); err != nil {
			return domain.LLMSettings{}, fmt.Errorf("seal key: %w", err)
		}
	}
	if err := s.d.Store.SaveCredential(ctx, user, port.StoredCredential{Settings: set, SealedKey: sealed}); err != nil {
		return domain.LLMSettings{}, err
	}
	return set, nil
}

func validKey(key string) error {
	if len(key) < 8 || len(key) > 512 {
		return fmt.Errorf("%w: the API key looks wrong (expected 8 to 512 characters)", domain.ErrInvalid)
	}
	for _, r := range key {
		if r > unicode.MaxASCII || unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("%w: the API key has spaces or unexpected characters", domain.ErrInvalid)
		}
	}
	return nil
}

// Test implements port.CredentialService: one short call with the saved
// key, so the user learns now whether it works rather than mid-report.
func (s *Service) Test(ctx context.Context, user domain.UserID) error {
	cred, _, err := s.CredentialFor(ctx, user)
	if err != nil {
		return err
	}
	llm, err := s.d.LLM.Connect(cred)
	if err != nil {
		return port.RedactKey(err, cred.APIKey)
	}
	_, err = llm.Generate(ctx, port.LLMRequest{Model: cred.Model, Prompt: "Reply with the single word OK.", MaxTokens: 512})
	if err != nil {
		return port.RedactKey(fmt.Errorf("%w: %w", domain.ErrUpstream, err), cred.APIKey)
	}
	return nil
}

// Delete implements port.CredentialService.
func (s *Service) Delete(ctx context.Context, user domain.UserID) error {
	if s.d.Box == nil {
		return errDisabled
	}
	return s.d.Store.DeleteCredential(ctx, user)
}

// CredentialFor implements port.CredentialSource.
func (s *Service) CredentialFor(ctx context.Context, user domain.UserID) (port.LLMCredential, domain.LLMSettings, error) {
	if s.d.Box == nil {
		return port.LLMCredential{}, domain.LLMSettings{}, errDisabled
	}
	c, err := s.d.Store.Credential(ctx, user)
	if err != nil {
		return port.LLMCredential{}, domain.LLMSettings{}, err
	}
	key, err := s.open(c, user)
	if err != nil {
		return port.LLMCredential{}, domain.LLMSettings{}, err
	}
	st := c.Settings
	return port.LLMCredential{Provider: st.Provider, Model: st.Model, APIKey: key, BaseURL: st.BaseURL}, st, nil
}

// open fails closed: a key sealed under another master key (rotated
// without re-wrapping) asks the user to save it again.
func (s *Service) open(c port.StoredCredential, user domain.UserID) (string, error) {
	plain, err := s.d.Box.Open(c.SealedKey, aad(user))
	if err != nil {
		return "", fmt.Errorf("%w: the saved API key can no longer be read; save it again", domain.ErrInvalid)
	}
	return string(plain), nil
}
