package b3api_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/b3api"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := base64.StdEncoding.DecodeString(path.Base(r.URL.Path))
		if err != nil {
			t.Errorf("param not base64: %v", err)
		}
		var q map[string]any
		_ = json.Unmarshal(raw, &q)
		switch path.Dir(r.URL.Path) {
		case "/indexProxy/indexCall/GetPortfolioDay":
			if q["index"] != "IBOV" {
				t.Errorf("index param = %v", q["index"])
			}
			if q["pageNumber"].(float64) == 1 {
				fmt.Fprint(w, `{"page":{"pageNumber":1,"pageSize":120,"totalRecords":3,"totalPages":2},
					"results":[{"cod":"PETR4","asset":"PETROBRAS","type":"PN  N2","part":"7,512"},
					           {"cod":"TAEE11","asset":"TAESA","type":"UNT N2","part":"0,401"}]}`)
				return
			}
			fmt.Fprint(w, `{"page":{"pageNumber":2,"totalPages":2},"results":[{"cod":"VALE3","asset":"VALE","type":"ON  NM"}]}`)
		case "/listedCompaniesProxy/CompanyCall/GetListedSupplementCompany":
			if q["issuingCompany"] != "PETR" {
				fmt.Fprint(w, `[]`)
				return
			}
			fmt.Fprint(w, `[{"code":"PETR",
				"cashDividends":[
					{"isinCode":"BRPETRACNPR6","rate":"0,54441118","label":"DIVIDENDO","lastDatePrior":"21/11/2024"},
					{"isinCode":"BRPETRACNPR6","rate":"0,10000000","label":"DIVIDENDO","lastDatePrior":"21/11/2024"},
					{"isinCode":"BRPETRACNPR6","rate":"0,20","label":"JRS CAP PROPRIO","lastDatePrior":"21/11/2024"},
					{"isinCode":"BRPETRACNOR9","rate":"9,99","label":"DIVIDENDO","lastDatePrior":"21/11/2024"},
					{"isinCode":"BRPETRACNPR6","rate":"1,00","label":"DIVIDENDO","lastDatePrior":"02/01/2019"}],
				"stockDividends":[
					{"isinCode":"BRPETRACNPR6","factor":"100,00000000000","label":"DESDOBRAMENTO","lastDatePrior":"25/04/2008"},
					{"isinCode":"BRPETRACNPR6","factor":"10,00","label":"BONIFICACAO","lastDatePrior":"03/03/2023"}]}]`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestConstituents(t *testing.T) {
	srv := server(t)
	defer srv.Close()
	got, err := b3api.New(srv.Client(), srv.URL).Constituents(context.Background(), domain.IndexIbovespa)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Key.Symbol != "PETR4" || got[1].Class != domain.ClassStock || got[2].Name != "VALE" {
		t.Fatalf("constituents = %+v", got)
	}
	if _, err := b3api.New(srv.Client(), srv.URL).Constituents(context.Background(), domain.IndexSP500); err == nil {
		t.Fatal("S&P 500 is not a B3 index")
	}
}

func TestCorporateActions(t *testing.T) {
	srv := server(t)
	defer srv.Close()
	c := b3api.New(srv.Client(), srv.URL)
	petr4 := domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}

	got, err := c.CorporateActions(context.Background(), petr4, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	ex := time.Date(2024, 11, 22, 0, 0, 0, 0, time.UTC)
	want := []domain.CorporateAction{
		{Asset: petr4, ExDate: time.Date(2023, 3, 4, 0, 0, 0, 0, time.UTC), Type: domain.ActionBonus, Value: 1.1, Source: "b3_api"},
		{Asset: petr4, ExDate: ex, Type: domain.ActionDividend, Value: 0.64441118, Source: "b3_api"},
		{Asset: petr4, ExDate: ex, Type: domain.ActionJCP, Value: 0.2, Source: "b3_api"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d actions: %+v", len(got), got)
	}
	// Same-day ordering between dividend and JCP is not specified.
	byType := map[domain.CorporateActionType]domain.CorporateAction{}
	for _, a := range got {
		byType[a.Type] = a
	}
	for _, w := range want {
		g := byType[w.Type]
		if !g.ExDate.Equal(w.ExDate) || g.Value-w.Value > 1e-9 || w.Value-g.Value > 1e-9 || g.Source != w.Source {
			t.Errorf("%s = %+v, want %+v", w.Type, g, w)
		}
	}

	if _, err := c.CorporateActions(context.Background(), domain.AssetKey{Market: domain.MarketB3, Symbol: "XXXX3"}, time.Time{}); err == nil {
		t.Error("unknown issuer should be not found")
	}
}
