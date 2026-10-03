package credential_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/secretbox"
	"github.com/CaioAP/shinrin/backend/internal/app/credential"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

const key = "sk-ant-test-0123456789abcd"

// fakeLLM answers or fails, and records the credential it was built with.
type fakeLLM struct {
	fail error
	got  []port.LLMCredential
}

func (f *fakeLLM) Connect(c port.LLMCredential) (port.LLMProvider, error) {
	if c.Provider != "anthropic" && c.Provider != "openai" {
		return nil, fmt.Errorf("%w: unknown provider", domain.ErrInvalid)
	}
	if c.Provider == "openai" && c.Model == "" {
		return nil, fmt.Errorf("%w: model needed", domain.ErrInvalid)
	}
	f.got = append(f.got, c)
	return f, nil
}
func (f *fakeLLM) Name() string { return "fake" }
func (f *fakeLLM) Generate(context.Context, port.LLMRequest) (port.LLMResponse, error) {
	return port.LLMResponse{Text: "OK"}, f.fail
}

func setup(t *testing.T) (*credential.Service, *memory.AccountStore, *fakeLLM, domain.UserID) {
	t.Helper()
	store := memory.NewAccountStore()
	u, err := store.CreateUser(context.Background(), "a@example.com", "h")
	if err != nil {
		t.Fatal(err)
	}
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	box, _ := secretbox.New(mk)
	llm := &fakeLLM{}
	return credential.New(credential.Deps{Store: store, Box: box, LLM: llm}, nil), store, llm, u.ID
}

func TestSaveSealsAndHidesKey(t *testing.T) {
	svc, store, _, u := setup(t)
	ctx := context.Background()
	set, err := svc.Save(ctx, u, port.CredentialInput{Provider: " Anthropic ", APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	if set.Provider != "anthropic" || set.KeyHint != "…abcd" || set.MonthlyCap != domain.DefaultMonthlyReportCap {
		t.Fatalf("settings = %+v", set)
	}
	stored, _ := store.Credential(ctx, u)
	if bytes.Contains(stored.SealedKey, []byte(key)) || strings.Contains(fmt.Sprintf("%+v", stored.Settings), key) {
		t.Fatal("key stored in the clear")
	}
	cred, _, err := svc.CredentialFor(ctx, u)
	if err != nil || cred.APIKey != key {
		t.Fatalf("CredentialFor = %v, %v", cred, err)
	}
	// A sealed key copied to another user's row does not open.
	other, _ := store.CreateUser(ctx, "b@example.com", "h")
	_ = store.SaveCredential(ctx, other.ID, stored)
	if _, _, err := svc.CredentialFor(ctx, other.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("copied key opened: %v", err)
	}
}

func TestSaveKeepsKeyOnlyForSameEndpoint(t *testing.T) {
	svc, _, _, u := setup(t)
	ctx := context.Background()
	if _, err := svc.Save(ctx, u, port.CredentialInput{Provider: "openai", Model: "gpt-x", BaseURL: "https://openrouter.ai/api/v1", APIKey: key}); err != nil {
		t.Fatal(err)
	}
	set, err := svc.Save(ctx, u, port.CredentialInput{Provider: "openai", Model: "gpt-y", BaseURL: "https://openrouter.ai/api/v1", MonthlyCap: 5})
	if err != nil || set.Model != "gpt-y" || set.MonthlyCap != 5 || set.KeyHint != "…abcd" {
		t.Fatalf("model-only update = %+v, %v", set, err)
	}
	if _, err := svc.Save(ctx, u, port.CredentialInput{Provider: "openai", Model: "gpt-y", BaseURL: "https://evil.example.com/v1"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("kept key sent to a new endpoint: %v", err)
	}
}

func TestSaveRejects(t *testing.T) {
	svc, _, _, u := setup(t)
	for name, in := range map[string]port.CredentialInput{
		"no key":          {Provider: "anthropic"},
		"short key":       {Provider: "anthropic", APIKey: "sk-1"},
		"spaces":          {Provider: "anthropic", APIKey: "sk ant 0123456789"},
		"provider":        {Provider: "gemini", APIKey: key},
		"openai no model": {Provider: "openai", APIKey: key},
		"private url":     {Provider: "openai", Model: "m", BaseURL: "https://169.254.169.254/", APIKey: key},
		"cap":             {Provider: "anthropic", APIKey: key, MonthlyCap: 10_000},
	} {
		if _, err := svc.Save(context.Background(), u, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		} else if strings.Contains(err.Error(), key) {
			t.Errorf("%s: error leaks the key", name)
		}
	}
}

func TestTestReportsUpstreamWithoutKey(t *testing.T) {
	svc, _, llm, u := setup(t)
	ctx := context.Background()
	if err := svc.Test(ctx, u); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Test without key = %v", err)
	}
	_, _ = svc.Save(ctx, u, port.CredentialInput{Provider: "anthropic", APIKey: key})
	if err := svc.Test(ctx, u); err != nil {
		t.Fatalf("Test = %v", err)
	}
	llm.fail = fmt.Errorf("HTTP 401: Incorrect API key provided: %s", key)
	err := svc.Test(ctx, u)
	if !errors.Is(err, domain.ErrUpstream) || strings.Contains(err.Error(), key) {
		t.Fatalf("Test with bad key = %v", err)
	}
}

func TestDisabledWithoutMasterKey(t *testing.T) {
	svc := credential.New(credential.Deps{Store: memory.NewAccountStore(), LLM: &fakeLLM{}}, nil)
	if _, err := svc.Save(context.Background(), 1, port.CredentialInput{Provider: "anthropic", APIKey: key}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("Save = %v", err)
	}
	if _, _, err := svc.CredentialFor(context.Background(), 1); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("CredentialFor = %v", err)
	}
}

func TestDelete(t *testing.T) {
	svc, _, _, u := setup(t)
	ctx := context.Background()
	_, _ = svc.Save(ctx, u, port.CredentialInput{Provider: "anthropic", APIKey: key})
	if err := svc.Delete(ctx, u); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Settings(ctx, u); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Settings after delete = %v", err)
	}
}
