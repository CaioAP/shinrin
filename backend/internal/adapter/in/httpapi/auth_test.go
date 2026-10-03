package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/httpapi"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/account"
	"github.com/CaioAP/shinrin/backend/internal/app/watchlist"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

type plainHasher struct{}

func (plainHasher) Hash(pw string) (string, error)       { return "h:" + pw, nil }
func (plainHasher) Verify(pw, hash string) (bool, error) { return hash == "h:"+pw, nil }

// accountServer wires the real account and watchlist services on memory
// stores, so these tests cover the HTTP contract end to end.
func accountServer(t *testing.T) http.Handler {
	t.Helper()
	assets, st := memory.NewAssetRepository(), memory.NewAccountStore()
	_ = assets.UpsertAssets(t.Context(), []domain.Asset{{Key: petr, Class: domain.ClassStock, Active: true}})
	return httpapi.NewRouter(httpapi.Deps{
		System: fakeSystem{healthy: true}, Catalog: &fakeCatalog{}, Analysis: &fakeAnalysis{}, Market: &fakeMarket{},
		Accounts: account.New(account.Deps{Users: st, Sessions: st, Hasher: plainHasher{}}, account.Options{}),
		Lists:    watchlist.New(watchlist.Deps{Lists: st, Assets: assets, Analysis: &fakeAnalysis{}}),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func send(t *testing.T, h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuthFlow(t *testing.T) {
	h := accountServer(t)
	rec := send(t, h, "POST", "/api/v1/auth/signup", "", `{"email":"caio@example.com","password":"correct horse battery"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup status %d: %s", rec.Code, rec.Body)
	}
	var sess struct {
		Token string `json:"token"`
		User  struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&sess)
	if sess.Token == "" || sess.User.Email != "caio@example.com" {
		t.Fatalf("session = %+v", sess)
	}

	tests := []struct {
		name, method, path, token, body string
		want                            int
	}{
		{"duplicate signup", "POST", "/api/v1/auth/signup", "", `{"email":"caio@example.com","password":"correct horse battery"}`, http.StatusConflict},
		{"weak password", "POST", "/api/v1/auth/signup", "", `{"email":"b@example.com","password":"short"}`, http.StatusBadRequest},
		{"unknown field", "POST", "/api/v1/auth/signup", "", `{"email":"b@example.com","password":"correct horse battery","admin":true}`, http.StatusBadRequest},
		{"wrong password", "POST", "/api/v1/auth/signin", "", `{"email":"caio@example.com","password":"wrong horse battery"}`, http.StatusUnauthorized},
		{"signin", "POST", "/api/v1/auth/signin", "", `{"email":"caio@example.com","password":"correct horse battery"}`, http.StatusOK},
		{"me without token", "GET", "/api/v1/me", "", "", http.StatusUnauthorized},
		{"me with bad token", "GET", "/api/v1/me", "nope", "", http.StatusUnauthorized},
		{"me", "GET", "/api/v1/me", sess.Token, "", http.StatusOK},
		{"incomplete questionnaire", "PUT", "/api/v1/me/risk-profile", sess.Token, `{"answers":{"horizon":"gt_5y"}}`, http.StatusBadRequest},
		{"questionnaire", "GET", "/api/v1/risk-questionnaire", "", "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := send(t, h, tt.method, tt.path, tt.token, tt.body); rec.Code != tt.want {
				t.Errorf("status %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}

	answers := map[string]string{}
	for _, q := range domain.SuitabilityQuestions {
		answers[q.ID] = q.Answers[0]
	}
	b, _ := json.Marshal(map[string]any{"answers": answers})
	rec = send(t, h, "PUT", "/api/v1/me/risk-profile", sess.Token, string(b))
	if rec.Code != http.StatusOK || decode(t, rec.Body)["profile"] != "conservative" {
		t.Errorf("risk profile status %d", rec.Code)
	}

	if rec := send(t, h, "POST", "/api/v1/auth/signout", sess.Token, ""); rec.Code != http.StatusNoContent {
		t.Errorf("signout status %d", rec.Code)
	}
	if rec := send(t, h, "GET", "/api/v1/me", sess.Token, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("me after signout status %d", rec.Code)
	}
}

func TestWatchlistEndpoints(t *testing.T) {
	h := accountServer(t)
	rec := send(t, h, "POST", "/api/v1/auth/signup", "", `{"email":"caio@example.com","password":"correct horse battery"}`)
	tok := decode(t, rec.Body)["token"].(string)

	if rec := send(t, h, "GET", "/api/v1/watchlists", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous list status %d", rec.Code)
	}
	rec = send(t, h, "POST", "/api/v1/watchlists", tok, `{"name":"Dividends"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, "PUT", "/api/v1/watchlists/1/items/B3/PETR4", tok, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("add status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, "PUT", "/api/v1/watchlists/1/items/B3/XXXX3", tok, ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown asset status %d", rec.Code)
	}
	if rec := send(t, h, "PUT", "/api/v1/watchlists/abc/items/B3/PETR4", tok, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id status %d", rec.Code)
	}

	rec = send(t, h, "GET", "/api/v1/watchlists/1?profile=aggressive", tok, "")
	body := decode(t, rec.Body)
	item := body["items"].([]any)[0].(map[string]any)
	if rec.Code != http.StatusOK || body["profile"] != "aggressive" || body["disclaimer"] != domain.Disclaimer || item["scores"] == nil {
		t.Errorf("entries = %d %v", rec.Code, body)
	}
	if rec := send(t, h, "PATCH", "/api/v1/watchlists/1", tok, `{"name":"Income"}`); rec.Code != http.StatusNoContent {
		t.Errorf("rename status %d", rec.Code)
	}
	if rec := send(t, h, "DELETE", "/api/v1/watchlists/1/items/B3/PETR4", tok, ""); rec.Code != http.StatusNoContent {
		t.Errorf("remove item status %d", rec.Code)
	}
	if rec := send(t, h, "DELETE", "/api/v1/watchlists/1", tok, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete status %d", rec.Code)
	}
	if rec := send(t, h, "DELETE", "/api/v1/me", tok, `{"password":"correct horse battery"}`); rec.Code != http.StatusNoContent {
		t.Errorf("delete account status %d", rec.Code)
	}
}
