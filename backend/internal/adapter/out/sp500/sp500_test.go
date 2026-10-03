package sp500_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/sp500"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

const csvBody = `Symbol,Security,GICS Sector,GICS Sub-Industry,Headquarters Location,Date added,CIK,Founded
AAPL,Apple Inc.,Information Technology,"Technology Hardware, Storage & Peripherals","Cupertino, California",1982-11-30,0000320193,1977
BRK.B,Berkshire Hathaway,Financials,Multi-Sector Holdings,"Omaha, Nebraska",2010-02-16,0001067983,1839
PLD,Prologis,Real Estate,Industrial REITs,"San Francisco, California",2003-07-17,1045609,1983
`

func TestConstituents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, csvBody) }))
	defer srv.Close()

	got, err := sp500.New(srv.Client(), srv.URL).Constituents(context.Background(), domain.IndexSP500)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d", len(got))
	}
	if a := got[0]; a.Key != (domain.AssetKey{Market: domain.MarketUS, Symbol: "AAPL"}) || a.CIK != "0000320193" || a.Sector != "Information Technology" || !a.IndexMember {
		t.Errorf("AAPL = %+v", a)
	}
	if got[1].Key.Symbol != "BRK.B" {
		t.Errorf("BRK.B = %+v", got[1])
	}
	if got[2].Class != domain.ClassREIT || got[2].CIK != "0001045609" {
		t.Errorf("PLD = %+v", got[2])
	}
	if _, err := sp500.New(srv.Client(), srv.URL).Constituents(context.Background(), domain.IndexIbovespa); err == nil {
		t.Error("IBOV should be rejected")
	}
}
