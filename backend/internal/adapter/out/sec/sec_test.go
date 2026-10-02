package sec_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/sec"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// A trimmed companyfacts document in EDGAR's shape.
const appleFacts = `{
 "cik": 320193, "entityName": "Apple Inc.",
 "facts": {
  "dei": {"EntityCommonStockSharesOutstanding": {"units": {"shares": [
    {"end": "2025-01-17", "val": 15037874000, "fy": 2025, "fp": "Q1", "form": "10-Q", "filed": "2025-01-31"}]}}},
  "us-gaap": {
   "RevenueFromContractWithCustomerExcludingAssessedTax": {"units": {"USD": [
    {"start": "2024-09-29", "end": "2024-12-28", "val": 124300000000, "form": "10-Q", "filed": "2025-01-31"},
    {"start": "2023-10-01", "end": "2024-09-28", "val": 391035000000, "form": "10-K", "filed": "2024-11-01"},
    {"start": "2024-03-31", "end": "2024-06-29", "val": 85777000000, "form": "10-Q", "filed": "2024-08-02"},
    {"start": "2023-10-01", "end": "2024-06-29", "val": 296105000000, "form": "10-Q", "filed": "2024-08-02"},
    {"start": "2024-09-29", "end": "2024-12-28", "val": 1, "form": "8-K", "filed": "2025-01-30"}]}},
   "Revenues": {"units": {"USD": [
    {"start": "2024-09-29", "end": "2024-12-28", "val": 999, "form": "10-Q", "filed": "2025-01-31"},
    {"start": "2014-09-28", "end": "2015-09-26", "val": 233715000000, "form": "10-K", "filed": "2015-10-28"}]}},
   "NetIncomeLoss": {"units": {"USD": [
    {"start": "2023-10-01", "end": "2024-09-28", "val": 93736000000, "form": "10-K", "filed": "2024-11-01"},
    {"start": "2023-10-01", "end": "2024-09-28", "val": 93700000000, "form": "10-K/A", "filed": "2025-02-01"}]}},
   "StockholdersEquity": {"units": {"USD": [
    {"end": "2024-09-28", "val": 56950000000, "form": "10-K", "filed": "2024-11-01"},
    {"end": "2024-12-28", "val": 66758000000, "form": "10-Q", "filed": "2025-01-31"}]}},
   "LongTermDebtNoncurrent": {"units": {"USD": [{"end": "2024-12-28", "val": 83956000000, "form": "10-Q", "filed": "2025-01-31"}]}},
   "LongTermDebtCurrent": {"units": {"USD": [{"end": "2024-12-28", "val": 10848000000, "form": "10-Q", "filed": "2025-01-31"}]}},
   "CommercialPaper": {"units": {"USD": [{"end": "2024-12-28", "val": 1995000000, "form": "10-Q", "filed": "2025-01-31"}]}}
  }
 }
}`

func TestFundamentals(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/files/company_tickers.json":
			fmt.Fprint(w, `{"0":{"cik_str":320193,"ticker":"AAPL","title":"Apple Inc."},"1":{"cik_str":1067983,"ticker":"BRK-B","title":"Berkshire"}}`)
		case "/api/xbrl/companyfacts/CIK0000320193.json":
			fmt.Fprint(w, appleFacts)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := sec.New(srv.Client(), srv.URL, srv.URL+"/files/company_tickers.json")
	aapl := domain.AssetKey{Market: domain.MarketUS, Symbol: "AAPL"}
	rows, err := c.Fundamentals(context.Background(), aapl, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	type k struct {
		metric string
		end    string
		typ    domain.PeriodType
	}
	got := map[k]float64{}
	for _, r := range rows {
		if r.Asset != aapl || r.Source != "sec_edgar" {
			t.Errorf("row %+v", r)
		}
		got[k{r.Metric, r.PeriodEnd.Format(time.DateOnly), r.PeriodType}] = r.Value
	}
	want := map[k]float64{
		{domain.MetricRevenue, "2024-12-28", domain.PeriodQuarter}:      124300000000, // preferred tag beats Revenues
		{domain.MetricRevenue, "2024-09-28", domain.PeriodFiscalYear}:   391035000000,
		{domain.MetricRevenue, "2024-06-29", domain.PeriodQuarter}:      85777000000, // 9-month YTD dropped
		{domain.MetricNetIncome, "2024-09-28", domain.PeriodFiscalYear}: 93700000000, // amended filing wins
		{domain.MetricEquity, "2024-09-28", domain.PeriodFiscalYear}:    56950000000,
		{domain.MetricEquity, "2024-12-28", domain.PeriodQuarter}:       66758000000,
		{domain.MetricGrossDebt, "2024-12-28", domain.PeriodQuarter}:    83956000000 + 10848000000 + 1995000000,
		{domain.MetricShares, "2025-01-17", domain.PeriodQuarter}:       15037874000,
	}
	for key, w := range want {
		if g, ok := got[key]; !ok || g != w {
			t.Errorf("%+v = %v (present %v), want %v", key, g, ok, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d rows, want %d (2015 revenue is before since): %v", len(got), len(want), got)
	}

	_, err = c.Fundamentals(context.Background(), domain.AssetKey{Market: domain.MarketUS, Symbol: "BRK.B"}, time.Time{})
	if !errors.Is(err, domain.ErrNotFound) { // ticker known (dash mapped to dot), facts 404 on the fake
		t.Errorf("BRK.B err = %v", err)
	}
	if _, err := c.Fundamentals(context.Background(), domain.AssetKey{Market: domain.MarketUS, Symbol: "NOPE"}, time.Time{}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown ticker err = %v", err)
	}
	n := 0
	for _, p := range paths {
		if p == "/files/company_tickers.json" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("ticker list fetched %d times, want once", n)
	}
}
