package cvm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/cvm"
)

func TestNews(t *testing.T) {
	files := fixtures(t)
	files["/IPE/DADOS/ipe_cia_aberta_2025.zip"] = zipFiles(t, map[string][]string{
		"ipe_cia_aberta_2025.csv": {
			"CNPJ_Companhia;Nome_Companhia;Codigo_CVM;Data_Referencia;Categoria;Tipo;Especie;Assunto;Data_Entrega;Tipo_Apresentacao;Protocolo_Entrega;Versao;Link_Download",
			strings.Join([]string{petro, "PETRÓLEO BRASILEIRO S.A. - PETROBRAS", "9512", "2025-08-01", "Fato Relevante", "", "", "Aprovação de dividendos", "2025-08-07", "AP", "1", "1", "https://www.rad.cvm.gov.br/a"}, ";"),
			strings.Join([]string{petro, "PETROBRAS", "9512", "2025-08-01", "Calendário de Eventos Corporativos", "", "", "", "2025-08-07", "AP", "2", "1", "https://www.rad.cvm.gov.br/b"}, ";"),
			strings.Join([]string{petro, "PETROBRAS", "9512", "2025-01-01", "Fato Relevante", "", "", "Old", "2025-01-05", "AP", "3", "1", "https://www.rad.cvm.gov.br/c"}, ";"),
			strings.Join([]string{"99.999.999/0001-99", "UNLISTED", "1", "2025-08-01", "Fato Relevante", "", "", "X", "2025-08-07", "AP", "4", "1", "https://www.rad.cvm.gov.br/d"}, ";"),
		},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	defer srv.Close()

	c := cvm.New(srv.Client(), srv.URL, func() time.Time { return time.Date(2025, 8, 20, 0, 0, 0, 0, time.UTC) })
	got, err := c.News(context.Background(), time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d items: %+v", len(got), got)
	}
	n := got[0]
	if n.Title != "Fato Relevante: Aprovação de dividendos" || n.Summary != "PETRÓLEO BRASILEIRO S.A. - PETROBRAS" || n.Lang != "pt" || n.URL != "https://www.rad.cvm.gov.br/a" {
		t.Errorf("item = %+v (Latin-1 must decode to UTF-8)", n)
	}
	if len(n.Assets) != 2 { // PETR3 and PETR4
		t.Errorf("assets = %v", n.Assets)
	}
}
