package cvm_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/cvm"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

const (
	petro = "33.000.167/0001-01"
	bank  = "60.872.504/0001-23"
	hdr   = "CNPJ_CIA;DT_REFER;VERSAO;DENOM_CIA;CD_CVM;GRUPO_DFP;MOEDA;ESCALA_MOEDA;ORDEM_EXERC;DT_INI_EXERC;DT_FIM_EXERC;CD_CONTA;DS_CONTA;VL_CONTA;ST_CONTA_FIXA"
	bsHdr = "CNPJ_CIA;DT_REFER;VERSAO;DENOM_CIA;CD_CVM;GRUPO_DFP;MOEDA;ESCALA_MOEDA;ORDEM_EXERC;DT_FIM_EXERC;CD_CONTA;DS_CONTA;VL_CONTA;ST_CONTA_FIXA"
	capH  = "CNPJ_CIA;DENOM_CIA;VERSAO;DT_REFER;QT_ACAO_ORDIN_CAP_INTEGR;QT_ACAO_PREF_CAP_INTEGR;QT_ACAO_TOTAL_CAP_INTEGR;QT_ACAO_ORDIN_TESOURO;QT_ACAO_PREF_TESOURO;QT_ACAO_TOTAL_TESOURO"
)

// latin1 encodes like CVM's files do, so accented ORDEM_EXERC values are not
// valid UTF-8, as in production.
func latin1(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteByte(byte(r))
	}
	return b.String()
}

func flow(cnpj, refer, ver, ordem, start, end, code, desc, val string) string {
	return strings.Join([]string{cnpj, refer, ver, "PETROBRAS", "9512", "DF Consolidado", "REAL", "MIL", ordem, start, end, code, desc, val, "S"}, ";")
}

func bal(cnpj, refer, ver, ordem, end, code, desc, val string) string {
	return strings.Join([]string{cnpj, refer, ver, "PETROBRAS", "9512", "DF Consolidado", "REAL", "MIL", ordem, end, code, desc, val, "S"}, ";")
}

func zipFiles(t *testing.T, files map[string][]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, lines := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(latin1(strings.Join(lines, "\r\n") + "\r\n")))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func fixtures(t *testing.T) map[string][]byte {
	const last, prev = "ÚLTIMO", "PENÚLTIMO"
	dfp := zipFiles(t, map[string][]string{
		"dfp_cia_aberta_DRE_con_2024.csv": {hdr,
			flow(petro, "2024-12-31", "1", last, "2024-01-01", "2024-12-31", "3.01", "Receita de Venda de Bens e/ou Serviços", "490000000"),
			flow(petro, "2024-12-31", "1", prev, "2023-01-01", "2023-12-31", "3.01", "Receita de Venda de Bens e/ou Serviços", "511000000"),
			flow(petro, "2024-12-31", "1", last, "2024-01-01", "2024-12-31", "3.05", "Resultado Antes do Resultado Financeiro e dos Tributos", "150000000"),
			flow(petro, "2024-12-31", "1", last, "2024-01-01", "2024-12-31", "3.11", "Lucro/Prejuízo Consolidado do Período", "36600000"),
			flow(petro, "2024-12-31", "1", last, "2024-01-01", "2024-12-31", "3.11.01", "Atribuído a Sócios da Empresa Controladora", "36000000"),
			flow(bank, "2024-12-31", "1", last, "2024-01-01", "2024-12-31", "3.01", "Receitas da Intermediação Financeira", "200000000"),
		},
		"dfp_cia_aberta_DFC_MI_con_2024.csv": {hdr,
			flow(petro, "2024-12-31", "1", last, "2024-01-01", "2024-12-31", "6.01", "Caixa Líquido Atividades Operacionais", "190000000"),
			flow(petro, "2024-12-31", "1", last, "2024-01-01", "2024-12-31", "6.01.01.02", "Depreciação, depleção e amortização", "60000000"),
			flow(petro, "2024-12-31", "1", last, "2024-01-01", "2024-12-31", "6.02.03", "Aquisições de ativos imobilizados e intangíveis", "-80000000"),
			flow(petro, "2024-12-31", "1", last, "2024-01-01", "2024-12-31", "6.03.05", "Dividendos pagos a acionistas da Petrobras", "-70000000"),
		},
		"dfp_cia_aberta_BPA_con_2024.csv": {bsHdr,
			bal(petro, "2024-12-31", "1", last, "2024-12-31", "1", "Ativo Total", "1100000000"),
			bal(petro, "2024-12-31", "1", last, "2024-12-31", "1.01.01", "Caixa e Equivalentes de Caixa", "40000000"),
			bal(petro, "2024-12-31", "1", last, "2024-12-31", "1.01.02", "Aplicações Financeiras", "20000000"),
			bal(petro, "2024-12-31", "1", prev, "2023-12-31", "1", "Ativo Total", "1000000000"),
		},
		"dfp_cia_aberta_BPP_con_2024.csv": {bsHdr,
			bal(petro, "2024-12-31", "1", last, "2024-12-31", "2.01.04", "Empréstimos e Financiamentos", "30000000"),
			bal(petro, "2024-12-31", "1", last, "2024-12-31", "2.02.01", "Empréstimos e Financiamentos", "999"),
			bal(petro, "2024-12-31", "1", last, "2024-12-31", "2.03", "Patrimônio Líquido Consolidado", "400000000"),
			bal(petro, "2024-12-31", "1", last, "2024-12-31", "2.03.09", "Participação dos Acionistas Não Controladores", "5000000"),
			// A resubmission (version 2) replaces all of version 1.
			bal(petro, "2024-12-31", "2", last, "2024-12-31", "2.01.04", "Empréstimos e Financiamentos", "30000000"),
			bal(petro, "2024-12-31", "2", last, "2024-12-31", "2.02.01", "Empréstimos e Financiamentos", "250000000"),
			bal(petro, "2024-12-31", "2", last, "2024-12-31", "2.03", "Patrimônio Líquido Consolidado", "410000000"),
			bal(petro, "2024-12-31", "2", last, "2024-12-31", "2.03.09", "Participação dos Acionistas Não Controladores", "5000000"),
		},
		"dfp_cia_aberta_composicao_capital_2024.csv": {capH,
			strings.Join([]string{petro, "PETROBRAS", "1", "2024-12-31", "7442231223", "5446501379", "12888732602", "0", "0", "0"}, ";")},
	})
	// 2025 ITR: Q1 and Q2. DRE has 3-month rows (and the Q1 YTD repeat);
	// cash flow is year to date only.
	itr := zipFiles(t, map[string][]string{
		"itr_cia_aberta_DRE_con_2025.csv": {hdr,
			flow(petro, "2025-03-31", "1", last, "2025-01-01", "2025-03-31", "3.01", "Receita", "123000000"),
			flow(petro, "2025-03-31", "1", last, "2025-01-01", "2025-03-31", "3.01", "Receita", "123000000"),
			flow(petro, "2025-06-30", "1", last, "2025-04-01", "2025-06-30", "3.01", "Receita", "119000000"),
			flow(petro, "2025-06-30", "1", last, "2025-01-01", "2025-06-30", "3.01", "Receita", "242000000"),
		},
		"itr_cia_aberta_DFC_MI_con_2025.csv": {hdr,
			flow(petro, "2025-03-31", "1", last, "2025-01-01", "2025-03-31", "6.01", "Caixa Líquido Atividades Operacionais", "50000000"),
			flow(petro, "2025-06-30", "1", last, "2025-01-01", "2025-06-30", "6.01", "Caixa Líquido Atividades Operacionais", "95000000"),
		},
		"itr_cia_aberta_BPP_ind_2025.csv": {bsHdr, // individual only: no consolidated BPP for this company this year
			bal(petro, "2025-06-30", "1", last, "2025-06-30", "2.03", "Patrimônio Líquido", "420000000"),
		},
	})
	fca := zipFiles(t, map[string][]string{
		"fca_cia_aberta_valor_mobiliario_2025.csv": {
			"CNPJ_Companhia;Data_Referencia;Versao;ID_Documento;Valor_Mobiliario;Sigla_Classe_Acao_Preferencial;Classe_Acao_Preferencial;Codigo_Negociacao;Composicao_BDR_Unit;Mercado",
			petro + ";2025-01-01;1;1;Ações Ordinárias;;;PETR3;;Bolsa",
			petro + ";2025-01-01;1;1;Ações Preferenciais;PN;Preferencial;PETR4;;Bolsa",
			bank + ";2025-01-01;1;2;Ações Preferenciais;PN;Preferencial;ITUB4;;Bolsa",
		},
	})
	return map[string][]byte{
		"/DFP/DADOS/dfp_cia_aberta_2024.zip": dfp,
		"/ITR/DADOS/itr_cia_aberta_2025.zip": itr,
		"/FCA/DADOS/fca_cia_aberta_2025.zip": fca,
	}
}

func TestFundamentals(t *testing.T) {
	files := fixtures(t)
	requests := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path]++
		b, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	defer srv.Close()

	now := time.Date(2025, 8, 20, 0, 0, 0, 0, time.UTC)
	c := cvm.New(srv.Client(), srv.URL, func() time.Time { return now })
	petr4 := domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}
	rows, err := c.Fundamentals(context.Background(), petr4, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
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
		got[k{r.Metric, r.PeriodEnd.Format(time.DateOnly), r.PeriodType}] = r.Value
	}
	FY, Q := domain.PeriodFiscalYear, domain.PeriodQuarter
	want := map[k]float64{
		{domain.MetricRevenue, "2024-12-31", FY}:           490e9,
		{domain.MetricEBIT, "2024-12-31", FY}:              150e9,
		{domain.MetricNetIncome, "2024-12-31", FY}:         36e9, // owners of the parent
		{domain.MetricOperatingCashFlow, "2024-12-31", FY}: 190e9,
		{domain.MetricDepreciation, "2024-12-31", FY}:      60e9,
		{domain.MetricCapex, "2024-12-31", FY}:             80e9,
		{domain.MetricDividendsPaid, "2024-12-31", FY}:     70e9,
		{domain.MetricTotalAssets, "2024-12-31", FY}:       1100e9,
		{domain.MetricCash, "2024-12-31", FY}:              60e9,
		{domain.MetricGrossDebt, "2024-12-31", FY}:         280e9,
		{domain.MetricEquity, "2024-12-31", FY}:            405e9, // version 2, minus minorities
		{domain.MetricShares, "2024-12-31", FY}:            12888732602,
		{domain.MetricRevenue, "2025-03-31", Q}:            123e9, // not doubled by the YTD repeat
		{domain.MetricRevenue, "2025-06-30", Q}:            119e9,
		{domain.MetricOperatingCashFlow, "2025-03-31", Q}:  50e9,
		{domain.MetricOperatingCashFlow, "2025-06-30", Q}:  45e9, // H1 minus Q1
		{domain.MetricEquity, "2025-06-30", Q}:             420e9,
	}
	for key, w := range want {
		if g, ok := got[key]; !ok || g != w {
			t.Errorf("%+v = %v (present %v), want %v", key, g, ok, w)
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("unexpected %+v = %v", key, got[key])
		}
	}

	// Second ticker of the same company: served from cache.
	if _, err := c.Fundamentals(context.Background(), domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR3"}, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	for path, n := range requests {
		if n > 1 {
			t.Errorf("%s downloaded %d times", path, n)
		}
	}

	_, err = c.Fundamentals(context.Background(), domain.AssetKey{Market: domain.MarketB3, Symbol: "ITUB4"}, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("bank err = %v, want ErrNotFound", err)
	}
	_, err = c.Fundamentals(context.Background(), domain.AssetKey{Market: domain.MarketB3, Symbol: "MXRF11"}, time.Time{})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown ticker err = %v", err)
	}
}
