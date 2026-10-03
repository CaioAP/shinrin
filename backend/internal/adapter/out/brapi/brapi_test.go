package brapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/brapi"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func TestQuotes(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/quote/PETR4":
			fmt.Fprint(w, `{"results":[{"symbol":"PETR4","regularMarketPrice":37.5,"regularMarketChangePercent":-1.2,"regularMarketTime":"2025-03-10T20:07:00.000Z"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":true,"message":"Não encontramos a ação"}`)
		}
	}))
	defer srv.Close()

	c := brapi.New(srv.Client(), srv.URL, "tok", 1)
	got, err := c.Quotes(context.Background(), []domain.AssetKey{
		{Market: domain.MarketB3, Symbol: "PETR4"},
		{Market: domain.MarketB3, Symbol: "XXXX3"},
		{Market: domain.MarketUS, Symbol: "AAPL"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Price != 37.5 || got[0].ChangePct != -1.2 || got[0].AsOf.Hour() != 20 {
		t.Fatalf("quotes = %+v", got)
	}
	if len(paths) != 2 {
		t.Errorf("requests = %v, want one per B3 ticker", paths)
	}
}
