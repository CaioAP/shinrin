package fred_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/fred"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func TestSeries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/series/observations" || q.Get("series_id") != "DGS10" || q.Get("api_key") != "k" || q.Get("observation_start") != "2025-01-01" {
			t.Errorf("request %s", r.URL)
		}
		fmt.Fprint(w, `{"observations":[{"date":"2025-01-01","value":"."},{"date":"2025-01-02","value":"4.57"}]}`)
	}))
	defer srv.Close()

	got, err := fred.New(srv.Client(), srv.URL, "k").Series(context.Background(), domain.SeriesUST10Y, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Value != 4.57 || got[0].Series != "ust_10y" {
		t.Fatalf("points = %+v", got)
	}
}

func TestErrorsDoNotLeakKey(t *testing.T) {
	c := fred.New(&http.Client{Transport: failing{}}, "http://fred.invalid", "secret-key")
	_, err := c.Series(context.Background(), domain.SeriesUST10Y, time.Now())
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("err = %v", err)
	}
}

type failing struct{}

func (failing) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("dial failed")
}
