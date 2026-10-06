package bcb_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/bcb"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func TestSeriesChunksAndParses(t *testing.T) {
	var windows []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bcdata.sgs.432/dados" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		windows = append(windows, q.Get("dataInicial")+"-"+q.Get("dataFinal"))
		if q.Get("dataInicial") == "01/01/2014" {
			w.WriteHeader(http.StatusNotFound) // no data in the window
			return
		}
		fmt.Fprint(w, `[{"data":"02/01/2025","valor":"12.25"},{"data":"03/01/2025","valor":"12.25"}]`)
	}))
	defer srv.Close()

	c := bcb.New(srv.Client(), srv.URL)
	got, err := c.Series(context.Background(), domain.SeriesSelicTarget, time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(windows) < 3 || windows[0] != "01/01/2014-31/12/2018" {
		t.Errorf("windows = %v", windows)
	}
	if len(got) == 0 || got[0].Value != 12.25 || got[0].Series != "selic" || got[0].Date.Day() != 2 {
		t.Errorf("points = %+v", got)
	}
	if _, err := c.Series(context.Background(), "gdp", time.Now()); err == nil {
		t.Error("unknown series should fail")
	}
}

func TestSeriesRetriesHTMLPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html>busy</html>")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"data":"02/01/2025","valor":"12.25"}]`)
	}))
	defer srv.Close()

	c := bcb.New(srv.Client(), srv.URL)
	c.SetBackoff(time.Millisecond)
	got, err := c.Series(context.Background(), domain.SeriesSelicTarget, time.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(got) != 1 {
		t.Errorf("calls = %d, points = %d", calls, len(got))
	}
}

func TestSeriesGivesUpOnPersistentHTML(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html>busy</html>")
	}))
	defer srv.Close()

	c := bcb.New(srv.Client(), srv.URL)
	c.SetBackoff(time.Millisecond)
	_, err := c.Series(context.Background(), domain.SeriesSelicTarget, time.Now().AddDate(0, -1, 0))
	if err == nil || !strings.Contains(err.Error(), "non-JSON") {
		t.Errorf("err = %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}
