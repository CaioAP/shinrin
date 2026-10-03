package tesouro_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/tesouro"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

const csvBody = "Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha\r\n" +
	"Tesouro IPCA+;15/05/2035;10/03/2025;7,45;7,57;1.987,26;1.965,01;1.964,22\r\n" +
	"Tesouro IPCA+ com Juros Semestrais;15/08/2060;10/03/2025;7,20;7,32;4.012,10;3.950,00;3.949,00\r\n" +
	"Tesouro Selic;01/03/2027;10/03/2025;0,07;0,08;15.950,12;15.940,02;15.939,00\r\n" +
	"Tesouro Prefixado;01/01/2026;02/01/2020;6,10;6,22;700,00;690,00;690,00\r\n" +
	"Tesouro Novo Titulo;01/01/2030;10/03/2025;1;1;1;1;1\r\n"

func TestBondQuotes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(csvBody)) }))
	defer srv.Close()

	got, err := tesouro.New(srv.Client(), srv.URL).BondQuotes(context.Background(), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d quotes: %+v", len(got), got)
	}
	q := got[0]
	want := domain.BondQuote{
		Asset: domain.AssetKey{Market: domain.MarketB3, Symbol: "NTNBP-20350515"}, Name: "Tesouro IPCA+ 2035",
		Maturity: time.Date(2035, 5, 15, 0, 0, 0, 0, time.UTC), Date: time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
		BuyRate: 7.45, SellRate: 7.57, BuyPrice: 1987.26, SellPrice: 1965.01, Source: "tesouro_transparente",
	}
	if q != want {
		t.Errorf("quote = %+v\nwant    %+v", q, want)
	}
	if got[1].Asset.Symbol != "NTNB-20600815" || !strings.HasPrefix(string(got[2].Asset.Symbol), "LFT-") {
		t.Errorf("codes = %s, %s", got[1].Asset.Symbol, got[2].Asset.Symbol)
	}
}
