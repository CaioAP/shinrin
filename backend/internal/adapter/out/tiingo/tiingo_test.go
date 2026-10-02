package tiingo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/tiingo"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func TestPricesAndActionsShareOneRequest(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Token secret" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/tiingo/daily/aapl/prices":
			if r.URL.Query().Get("startDate") != "2020-08-28" {
				t.Errorf("startDate = %q", r.URL.Query().Get("startDate"))
			}
			fmt.Fprint(w, `[
			 {"date":"2020-08-28T00:00:00.000Z","open":504.05,"high":505.77,"low":498.31,"close":499.23,"volume":46907479,"adjClose":121.5,"divCash":0.0,"splitFactor":1.0},
			 {"date":"2020-08-31T00:00:00.000Z","open":127.58,"high":131.0,"low":126.0,"close":129.04,"volume":225702700,"adjClose":126.9,"divCash":0.0,"splitFactor":4.0},
			 {"date":"2020-11-06T00:00:00.000Z","open":118.32,"high":119.2,"low":116.13,"close":118.69,"volume":114457922,"adjClose":116.8,"divCash":0.205,"splitFactor":1.0}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"detail":"Error: Ticker 'NOPE' not found"}`)
		}
	}))
	defer srv.Close()

	c := tiingo.New(srv.Client(), srv.URL, "secret")
	aapl := domain.AssetKey{Market: domain.MarketUS, Symbol: "AAPL"}
	from := time.Date(2020, 8, 28, 0, 0, 0, 0, time.UTC)
	bars, err := c.DailyPrices(context.Background(), aapl, from, time.Date(2020, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 2 || bars[1].Close != 129.04 || bars[1].Volume != 225702700 || !bars[1].Date.Equal(time.Date(2020, 8, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("bars = %+v", bars)
	}

	acts, err := c.CorporateActions(context.Background(), aapl, from)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("HTTP calls = %d, want 1 (memo)", calls)
	}
	if len(acts) != 2 || acts[0].Type != domain.ActionSplit || acts[0].Value != 4 || acts[1].Type != domain.ActionDividend || acts[1].Value != 0.205 {
		t.Fatalf("actions = %+v", acts)
	}

	_, err = c.DailyPrices(context.Background(), domain.AssetKey{Market: domain.MarketUS, Symbol: "NOPE"}, from, from)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown ticker err = %v", err)
	}
}
