package bcb_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
