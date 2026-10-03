package finnhub_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/finnhub"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func server(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Finnhub-Token") != "k" || r.URL.Query().Get("token") != "" {
			t.Errorf("token must go in the header only")
		}
		switch r.URL.Path {
		case "/quote":
			if r.URL.Query().Get("symbol") == "AAPL" {
				fmt.Fprint(w, `{"c":227.48,"d":1.2,"dp":0.53,"h":228,"l":225,"o":226,"pc":226.28,"t":1741636800}`)
				return
			}
			fmt.Fprint(w, `{"c":0,"d":null,"dp":null,"h":0,"l":0,"o":0,"pc":0,"t":0}`)
		case "/company-news":
			if r.URL.Query().Get("from") != "2025-03-01" {
				t.Errorf("from = %s", r.URL.Query().Get("from"))
			}
			fmt.Fprint(w, `[
			 {"category":"company","datetime":1741600000,"headline":"Apple unveils thing","id":1,"related":"AAPL","source":"Reuters","summary":"Short.","url":"https://example.com/a"},
			 {"category":"company","datetime":1741600001,"headline":"","id":2,"related":"AAPL","source":"X","summary":"","url":"https://example.com/b"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestQuotes(t *testing.T) {
	srv := server(t)
	defer srv.Close()
	c := finnhub.New(srv.Client(), srv.URL, "k")
	aapl := domain.AssetKey{Market: domain.MarketUS, Symbol: "AAPL"}
	got, err := c.Quotes(context.Background(), []domain.AssetKey{aapl, {Market: domain.MarketUS, Symbol: "NOPE"}, {Market: domain.MarketB3, Symbol: "PETR4"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Price != 227.48 || got[0].ChangePct != 0.53 || !got[0].AsOf.Equal(time.Unix(1741636800, 0)) {
		t.Fatalf("quotes = %+v", got)
	}
}

func TestCompanyNews(t *testing.T) {
	srv := server(t)
	defer srv.Close()
	c := finnhub.New(srv.Client(), srv.URL, "k")
	aapl := domain.AssetKey{Market: domain.MarketUS, Symbol: "AAPL"}
	got, err := c.CompanyNews(context.Background(), aapl, time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Apple unveils thing" || got[0].Source != "finnhub:Reuters" || got[0].Assets[0] != aapl || got[0].Lang != "en" {
		t.Fatalf("news = %+v", got)
	}
	if _, err := c.CompanyNews(context.Background(), domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}, time.Time{}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("B3 err = %v", err)
	}
}
