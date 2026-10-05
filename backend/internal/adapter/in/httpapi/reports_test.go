package httpapi_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/httpapi"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/secretbox"
	"github.com/CaioAP/shinrin/backend/internal/app/account"
	"github.com/CaioAP/shinrin/backend/internal/app/credential"
	"github.com/CaioAP/shinrin/backend/internal/app/userreport"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

const testKey = "sk-ant-api03-secret-wxyz"

type stubLLM struct{ fail bool }

func (s *stubLLM) Connect(c port.LLMCredential) (port.LLMProvider, error) {
	if c.Provider != "anthropic" {
		return nil, domain.ErrInvalid
	}
	return s, nil
}
func (s *stubLLM) Name() string { return "anthropic" }
func (s *stubLLM) Generate(context.Context, port.LLMRequest) (port.LLMResponse, error) {
	if s.fail {
		return port.LLMResponse{}, errors.New("HTTP 401: invalid key " + testKey)
	}
	return port.LLMResponse{Text: "OK"}, nil
}

// stubReports stands in for app/report: it stores a fixed report.
type stubReports struct{ store *memory.AnalysisStore }

func (s stubReports) Generate(ctx context.Context, req port.ReportRequest) (domain.Report, error) {
	r := domain.Report{UserID: req.User, Kind: domain.ReportKindAsset, Asset: req.Asset, Profile: req.Profile, AsOf: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		Snapshot: domain.ReportSnapshot{Facts: map[string]float64{"pe": 4.1}, Headlines: []domain.Headline{{ID: "news:1", Title: "Petrobras pays dividends"}}},
		Output:   domain.ReportOutput{Summary: "Cheap.", CitedData: []string{"pe", "news:1"}, AllocationMinPct: 1, AllocationMaxPct: 4},
		Provider: "anthropic", Model: "m", CreatedAt: time.Now()}
	var err error
	r.ID, err = s.store.SaveReport(ctx, r)
	return r, err
}

func aiServer(t *testing.T, withBox bool, llm *stubLLM) http.Handler {
	t.Helper()
	st, reports := memory.NewAccountStore(), memory.NewAnalysisStore()
	var box port.SecretBox
	if withBox {
		k := make([]byte, 32)
		_, _ = rand.Read(k)
		box, _ = secretbox.New(k)
	}
	creds := credential.New(credential.Deps{Store: st, Box: box, LLM: llm}, nil)
	return httpapi.NewRouter(httpapi.Deps{
		System: fakeSystem{healthy: true}, Catalog: &fakeCatalog{}, Analysis: &fakeAnalysis{}, Market: &fakeMarket{},
		Accounts:    account.New(account.Deps{Users: st, Sessions: st, Hasher: plainHasher{}}, account.Options{}),
		Credentials: creds,
		Reports:     userreport.New(userreport.Deps{Credentials: creds, Reports: stubReports{reports}, Library: reports}, nil),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func signUp(t *testing.T, h http.Handler, email string) string {
	t.Helper()
	rec := send(t, h, "POST", "/api/v1/auth/signup", "", `{"email":"`+email+`","password":"correct horse battery"}`)
	var s struct{ Token string }
	_ = json.NewDecoder(rec.Body).Decode(&s)
	return s.Token
}

func TestAIFlow(t *testing.T) {
	llm := &stubLLM{}
	h := aiServer(t, true, llm)
	tok, other := signUp(t, h, "a@example.com"), signUp(t, h, "b@example.com")

	rec := send(t, h, "GET", "/api/v1/me/llm", tok, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"available":true,"configured":false`) {
		t.Fatalf("settings before save: %d %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, "POST", "/api/v1/assets/B3/PETR4/reports", tok, `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("generate without key: %d %s", rec.Code, rec.Body)
	}
	rec = send(t, h, "PUT", "/api/v1/me/llm", tok, `{"provider":"anthropic","apiKey":"`+testKey+`","monthlyCap":2}`)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), testKey) || !strings.Contains(rec.Body.String(), `"keyHint":"…wxyz"`) {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, "POST", "/api/v1/me/llm/test", tok, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("test: %d %s", rec.Code, rec.Body)
	}

	rec = send(t, h, "POST", "/api/v1/assets/B3/PETR4/reports", tok, `{"profile":"aggressive","lang":"pt-BR"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body)
	}
	var rep struct {
		ID         int64
		Profile    string
		Disclaimer string
		Cited      []struct {
			Key   string
			Value *float64
			Text  string
		}
	}
	_ = json.NewDecoder(rec.Body).Decode(&rep)
	if rep.Profile != "aggressive" || rep.Disclaimer == "" || len(rep.Cited) != 2 || *rep.Cited[0].Value != 4.1 || rep.Cited[1].Text != "Petrobras pays dividends" {
		t.Fatalf("report = %+v", rep)
	}

	for _, c := range []struct {
		name, method, path, token, body string
		want                            int
	}{
		{"list", "GET", "/api/v1/assets/B3/PETR4/reports", tok, "", http.StatusOK},
		{"get own", "GET", "/api/v1/reports/1", tok, "", http.StatusOK},
		{"get other's", "GET", "/api/v1/reports/1", other, "", http.StatusNotFound},
		{"signed out", "GET", "/api/v1/assets/B3/PETR4/reports", "", "", http.StatusUnauthorized},
		{"bad id", "GET", "/api/v1/reports/x", tok, "", http.StatusBadRequest},
		{"bad profile", "POST", "/api/v1/assets/B3/PETR4/reports", tok, `{"profile":"yolo"}`, http.StatusBadRequest},
		{"second report", "POST", "/api/v1/assets/B3/PETR4/reports", tok, `{}`, http.StatusCreated},
		{"over cap", "POST", "/api/v1/assets/B3/PETR4/reports", tok, `{}`, http.StatusTooManyRequests},
		{"private base url", "PUT", "/api/v1/me/llm", tok, `{"provider":"anthropic","baseUrl":"https://127.0.0.1/","apiKey":"` + testKey + `"}`, http.StatusBadRequest},
	} {
		if rec := send(t, h, c.method, c.path, c.token, c.body); rec.Code != c.want {
			t.Errorf("%s: status %d, want %d: %s", c.name, rec.Code, c.want, rec.Body)
		}
	}
	if rec := send(t, h, "GET", "/api/v1/me/llm", tok, ""); !strings.Contains(rec.Body.String(), `"used":2,"cap":2`) {
		t.Errorf("usage: %s", rec.Body)
	}

	llm.fail = true
	rec = send(t, h, "POST", "/api/v1/me/llm/test", tok, "")
	if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), testKey) {
		t.Fatalf("failing test: %d %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, "DELETE", "/api/v1/me/llm", tok, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
}

func TestAIDisabledWithoutMasterKey(t *testing.T) {
	h := aiServer(t, false, &stubLLM{})
	tok := signUp(t, h, "a@example.com")
	if rec := send(t, h, "GET", "/api/v1/me/llm", tok, ""); !strings.Contains(rec.Body.String(), `"available":false`) {
		t.Fatalf("settings: %s", rec.Body)
	}
	if rec := send(t, h, "PUT", "/api/v1/me/llm", tok, `{"provider":"anthropic","apiKey":"`+testKey+`"}`); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
}
