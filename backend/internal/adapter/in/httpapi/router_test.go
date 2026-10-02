package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/httpapi"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// fakeSystem and fakeCatalog stand in for the application services, so these
// tests cover only the adapter's job: routing, parsing and rendering.
type fakeSystem struct{ healthy bool }

func (f fakeSystem) Info(context.Context) port.SystemInfo {
	return port.SystemInfo{Name: "shinrin", Version: "test", Disclaimer: domain.Disclaimer}
}

func (f fakeSystem) Health(context.Context) port.HealthReport {
	return port.HealthReport{OK: f.healthy}
}

type fakeCatalog struct {
	gotFilter port.AssetFilter
}

func (f *fakeCatalog) ListAssets(_ context.Context, flt port.AssetFilter) ([]domain.Asset, error) {
	f.gotFilter = flt
	return []domain.Asset{{Key: domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}, Class: domain.ClassStock, Name: "Petrobras PN"}}, nil
}

func (f *fakeCatalog) GetAsset(_ context.Context, key domain.AssetKey) (domain.Asset, error) {
	if key.Symbol != "PETR4" {
		return domain.Asset{}, domain.ErrNotFound
	}
	return domain.Asset{Key: key, Class: domain.ClassStock, Name: "Petrobras PN"}, nil
}

func newServer(sys fakeSystem, cat *fakeCatalog) http.Handler {
	return httpapi.NewRouter(httpapi.Deps{
		System:  sys,
		Catalog: cat,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func do(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthz(t *testing.T) {
	if rec := do(t, newServer(fakeSystem{healthy: true}, &fakeCatalog{}), "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("healthy status = %d, want 200", rec.Code)
	}
	if rec := do(t, newServer(fakeSystem{healthy: false}, &fakeCatalog{}), "/healthz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("degraded status = %d, want 503", rec.Code)
	}
}

func TestMetaIncludesDisclaimer(t *testing.T) {
	rec := do(t, newServer(fakeSystem{healthy: true}, &fakeCatalog{}), "/api/v1/meta")
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["disclaimer"] != domain.Disclaimer {
		t.Fatalf("disclaimer missing from meta response: %v", body)
	}
}

func TestListAssetsParsesFilter(t *testing.T) {
	cat := &fakeCatalog{}
	rec := do(t, newServer(fakeSystem{healthy: true}, cat), "/api/v1/assets?market=b3&class=stock&indexMember=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	f := cat.gotFilter
	if f.Market != domain.MarketB3 || f.Class != domain.ClassStock || f.IndexMember == nil || !*f.IndexMember {
		t.Fatalf("filter = %+v", f)
	}
	var body struct {
		Items []struct {
			Symbol   string `json:"symbol"`
			Currency string `json:"currency"`
		} `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].Symbol != "PETR4" || body.Items[0].Currency != "BRL" {
		t.Fatalf("items = %+v", body.Items)
	}
}

func TestListAssetsRejectsBadFilter(t *testing.T) {
	for _, q := range []string{"market=LSE", "class=crypto", "indexMember=maybe"} {
		rec := do(t, newServer(fakeSystem{healthy: true}, &fakeCatalog{}), "/api/v1/assets?"+q)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rec.Code)
		}
	}
}

func TestGetAsset(t *testing.T) {
	h := newServer(fakeSystem{healthy: true}, &fakeCatalog{})
	if rec := do(t, h, "/api/v1/assets/B3/petr4"); rec.Code != http.StatusOK {
		t.Fatalf("known asset status = %d", rec.Code)
	}
	if rec := do(t, h, "/api/v1/assets/US/NOPE"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown asset status = %d, want 404", rec.Code)
	}
}
