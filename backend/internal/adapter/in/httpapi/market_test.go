package httpapi_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/httpapi"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

type fakeMarket struct {
	gotFrom  time.Time
	gotSince time.Time
	gotLimit int
}

var clock = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

func (f *fakeMarket) PriceHistory(_ context.Context, k domain.AssetKey, from time.Time) ([]domain.PriceBar, error) {
	f.gotFrom = from
	if k != petr {
		return nil, domain.ErrNotFound
	}
	return []domain.PriceBar{{Asset: k, Date: clock.Truncate(24 * time.Hour), Close: 38, AdjClose: 37.5, Volume: 10, Source: "b3_cotahist"}}, nil
}

func (f *fakeMarket) Dividends(_ context.Context, k domain.AssetKey, since time.Time) ([]domain.CorporateAction, error) {
	f.gotSince = since
	return []domain.CorporateAction{{Asset: k, ExDate: clock, Type: domain.ActionJCP, Value: 0.5, Source: "b3_api"}}, nil
}

func (f *fakeMarket) News(_ context.Context, k domain.AssetKey, limit int) ([]domain.NewsItem, error) {
	f.gotLimit = limit
	return []domain.NewsItem{{URL: "https://x", Title: "Fato relevante", Source: "cvm_ipe", PublishedAt: clock}}, nil
}

func (f *fakeMarket) Macro(context.Context) (domain.MacroStrip, error) {
	return domain.MacroStrip{Items: []domain.MacroIndicator{{Code: "selic", Value: 15, Unit: "pct_year", AsOf: clock, Source: "bcb"}}, Missing: []string{"ipca_12m"}}, nil
}

func marketServer(f *fakeMarket) http.Handler {
	return httpapi.NewRouter(httpapi.Deps{System: fakeSystem{healthy: true}, Catalog: &fakeCatalog{}, Analysis: &fakeAnalysis{}, Market: f,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return clock }})
}

func TestPricesEndpoint(t *testing.T) {
	tests := []struct {
		query    string
		status   int
		wantFrom time.Time
	}{
		{"", http.StatusOK, clock.AddDate(-1, 0, 0)},
		{"?range=3m", http.StatusOK, clock.AddDate(0, -3, 0)},
		{"?range=max", http.StatusOK, time.Time{}},
		{"?range=2w", http.StatusBadRequest, time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			f := &fakeMarket{}
			rec := do(t, marketServer(f), "/api/v1/assets/B3/PETR4/prices"+tt.query)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d", rec.Code, tt.status)
			}
			if tt.status == http.StatusOK && !f.gotFrom.Equal(tt.wantFrom) {
				t.Errorf("from = %v, want %v", f.gotFrom, tt.wantFrom)
			}
		})
	}

	body := decode(t, do(t, marketServer(&fakeMarket{}), "/api/v1/assets/B3/PETR4/prices").Body)
	bar := body["items"].([]any)[0].(map[string]any)
	if body["source"] != "b3_cotahist" || bar["date"] != "2026-10-03" || bar["adjClose"] != 37.5 {
		t.Errorf("body = %v", body)
	}
	if rec := do(t, marketServer(&fakeMarket{}), "/api/v1/assets/B3/VALE3/prices"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown asset status %d", rec.Code)
	}
}

func TestDividendsAndNewsLimits(t *testing.T) {
	f := &fakeMarket{}
	h := marketServer(f)
	if rec := do(t, h, "/api/v1/assets/B3/PETR4/dividends?years=10"); rec.Code != http.StatusOK || !f.gotSince.Equal(clock.AddDate(-10, 0, 0)) {
		t.Fatalf("dividends status %d since %v", rec.Code, f.gotSince)
	}
	if rec := do(t, h, "/api/v1/assets/B3/PETR4/dividends?years=0"); rec.Code != http.StatusBadRequest {
		t.Errorf("years=0 status %d", rec.Code)
	}
	rec := do(t, h, "/api/v1/assets/B3/PETR4/news")
	if rec.Code != http.StatusOK || f.gotLimit != 20 {
		t.Fatalf("news status %d limit %d", rec.Code, f.gotLimit)
	}
	item := decode(t, rec.Body)["items"].([]any)[0].(map[string]any)
	if item["publishedAt"] != "2026-10-03T12:00:00Z" || item["source"] != "cvm_ipe" {
		t.Errorf("news item = %v", item)
	}
	if rec := do(t, h, "/api/v1/assets/B3/PETR4/news?limit=500"); rec.Code != http.StatusBadRequest {
		t.Errorf("limit=500 status %d", rec.Code)
	}
}

func TestMacroEndpoint(t *testing.T) {
	body := decode(t, do(t, marketServer(&fakeMarket{}), "/api/v1/macro").Body)
	item := body["items"].([]any)[0].(map[string]any)
	if item["code"] != "selic" || item["asOf"] != "2026-10-03" || body["missing"].([]any)[0] != "ipca_12m" {
		t.Errorf("body = %v", body)
	}
}
