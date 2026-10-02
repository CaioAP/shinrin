package cvm

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// filing is one parsed year of DFP or ITR, reduced to mapped metrics.
type filing struct {
	values    map[string][]value // CNPJ -> metric values
	financial map[string]bool    // CNPJs that file a bank or insurer chart
}

type value struct {
	metric string
	end    time.Time
	typ    domain.PeriodType
	value  float64
}

// line is one row of a statement CSV, already filtered to mapped accounts.
type line struct {
	stmt        string // BPA, BPP, DRE, DFC_MI or capital
	cnpj, refer string
	version     int
	start, end  time.Time
	code, desc  string
	value       float64
}

// filingKey identifies one statement of one company's filing.
func (l line) filingKey() string { return l.stmt + "|" + l.cnpj + "|" + l.refer }

// Statement files inside the zip, e.g. dfp_cia_aberta_DRE_con_2024.csv.
var statements = []string{"BPA", "BPP", "DRE", "DFC_MI"}

func parseFiling(body []byte, typ domain.PeriodType) (*filing, error) {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	// Consolidated statements win; a company without subsidiaries files only
	// the individual ones.
	byScope := map[string][]line{}
	var capital []line
	for _, zf := range zr.File {
		name := zf.Name
		switch {
		case strings.Contains(name, "_composicao_capital_"):
			ls, err := readCSV(zf, "capital", capitalRow)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			capital = append(capital, ls...)
		default:
			for _, st := range statements {
				for _, scope := range []string{"con", "ind"} {
					if strings.Contains(name, "_"+st+"_"+scope+"_") {
						ls, err := readCSV(zf, st, statementRow)
						if err != nil {
							return nil, fmt.Errorf("%s: %w", name, err)
						}
						byScope[scope] = append(byScope[scope], ls...)
					}
				}
			}
		}
	}

	hasCon := map[string]bool{}
	for _, l := range byScope["con"] {
		hasCon[l.filingKey()] = true
	}
	lines := byScope["con"]
	for _, l := range byScope["ind"] {
		if !hasCon[l.filingKey()] {
			lines = append(lines, l)
		}
	}
	lines = append(lines, capital...)
	return reduce(latestVersion(lines), typ), nil
}

// latestVersion drops rows from superseded resubmissions of a filing, and
// exact repeats of an account for the same period (a first-quarter ITR can
// list the quarter and the year to date, which are the same three months).
func latestVersion(ls []line) []line {
	latest := map[string]int{}
	for _, l := range ls {
		latest[l.filingKey()] = max(latest[l.filingKey()], l.version)
	}
	type dup struct {
		cnpj, refer, code string
		start, end        time.Time
	}
	seen := map[dup]bool{}
	out := ls[:0]
	for _, l := range ls {
		d := dup{l.cnpj, l.refer, l.code, l.start, l.end}
		if l.version == latest[l.filingKey()] && !seen[d] {
			seen[d] = true
			out = append(out, l)
		}
	}
	return out
}

// reduce maps accounts to metrics.
//
// Fixed accounts (same code for every non-financial company):
//
//	1        total assets          3.01   revenue
//	1.01.01  cash and equivalents  3.05   EBIT
//	1.01.02  financial investments 3.11   net income (3.11.01: owners of the parent)
//	2.01.04  short-term loans      6.01   operating cash flow
//	2.02.01  long-term loans
//	2.03     equity (2.03.09: non-controlling interests, subtracted)
//
// Depreciation, capex and dividends paid have company-specific sub-accounts,
// so they are matched by description within their section.
func reduce(ls []line, typ domain.PeriodType) *filing {
	f := &filing{values: map[string][]value{}, financial: map[string]bool{}}

	type stockKey struct {
		cnpj string
		end  time.Time
	}
	type flowKey struct {
		cnpj       string
		start, end time.Time
	}
	stocks := map[stockKey]map[string]float64{}
	flows := map[flowKey]map[string]float64{}
	add := func(m map[string]float64, k string, v float64) { m[k] += v }

	for _, l := range ls {
		if l.code == "3.01" && strings.Contains(strings.ToLower(l.desc), "intermedia") {
			f.financial[l.cnpj] = true // "Receitas da Intermediação Financeira"
		}
		if l.start.IsZero() { // balance sheet or capital composition
			k := stockKey{l.cnpj, l.end}
			if stocks[k] == nil {
				stocks[k] = map[string]float64{}
			}
			add(stocks[k], l.code, l.value)
			continue
		}
		k := flowKey{l.cnpj, l.start, l.end}
		if flows[k] == nil {
			flows[k] = map[string]float64{}
		}
		code := l.code
		desc := strings.ToLower(l.desc)
		switch {
		case strings.HasPrefix(code, "6.01.01.") && (strings.Contains(desc, "deprecia") || strings.HasPrefix(desc, "amortiza")):
			code = "da"
		case strings.HasPrefix(code, "6.02.") && strings.Contains(desc, "imobilizado") &&
			(strings.HasPrefix(desc, "aquisi") || strings.HasPrefix(desc, "adi") || strings.HasPrefix(desc, "compra") || strings.HasPrefix(desc, "investimento")):
			code = "capex"
		case strings.HasPrefix(code, "6.03.") && (strings.Contains(desc, "dividendo") || strings.Contains(desc, "juros sobre")):
			code = "dividends"
		}
		add(flows[k], code, l.value)
	}

	emit := func(cnpj string, end time.Time, t domain.PeriodType, metric string, v float64) {
		f.values[cnpj] = append(f.values[cnpj], value{metric: metric, end: end, typ: t, value: v})
	}

	for k, acc := range stocks {
		if v, ok := acc["1"]; ok {
			emit(k.cnpj, k.end, typ, domain.MetricTotalAssets, v)
		}
		if v, ok := sumOf(acc, "1.01.01", "1.01.02"); ok {
			emit(k.cnpj, k.end, typ, domain.MetricCash, v)
		}
		if v, ok := sumOf(acc, "2.01.04", "2.02.01"); ok {
			emit(k.cnpj, k.end, typ, domain.MetricGrossDebt, v)
		}
		if v, ok := acc["2.03"]; ok {
			emit(k.cnpj, k.end, typ, domain.MetricEquity, v-acc["2.03.09"])
		}
		if v, ok := acc["shares"]; ok && v > 0 {
			emit(k.cnpj, k.end, typ, domain.MetricShares, v)
		}
	}

	// Flows: annual filings report twelve months. Quarterly filings report
	// the quarter and the year to date for the income statement but only the
	// year to date for cash flows, so quarters are taken directly when
	// reported and otherwise derived from consecutive year-to-date values.
	metricOf := func(acc map[string]float64) map[string]float64 {
		m := map[string]float64{}
		set := func(name, code string, sign float64) {
			if v, ok := acc[code]; ok {
				m[name] = sign * v
			}
		}
		set(domain.MetricRevenue, "3.01", 1)
		set(domain.MetricEBIT, "3.05", 1)
		set(domain.MetricNetIncome, "3.11", 1)
		set(domain.MetricNetIncome, "3.11.01", 1) // overrides 3.11 when present
		set(domain.MetricOperatingCashFlow, "6.01", 1)
		set(domain.MetricDepreciation, "da", 1)
		set(domain.MetricCapex, "capex", -1)             // reported as an outflow
		set(domain.MetricDividendsPaid, "dividends", -1) // idem
		return m
	}
	type ytdKey struct {
		cnpj  string
		start time.Time
	}
	ytd := map[ytdKey]map[int]map[string]float64{} // months -> metrics
	quarters := map[stockKey]map[string]float64{}
	for k, acc := range flows {
		months := monthsBetween(k.start, k.end)
		m := metricOf(acc)
		switch {
		case typ == domain.PeriodFiscalYear && months == 12:
			for name, v := range m {
				emit(k.cnpj, k.end, domain.PeriodFiscalYear, name, v)
			}
		case typ == domain.PeriodQuarter && months == 3:
			q := stockKey{k.cnpj, k.end}
			if quarters[q] == nil {
				quarters[q] = map[string]float64{}
			}
			for name, v := range m {
				quarters[q][name] = v
			}
		}
		if typ == domain.PeriodQuarter && (months == 3 || months == 6 || months == 9) {
			yk := ytdKey{k.cnpj, k.start}
			if ytd[yk] == nil {
				ytd[yk] = map[int]map[string]float64{}
			}
			ytd[yk][months] = m
		}
	}
	for yk, byMonths := range ytd {
		for _, months := range []int{6, 9} {
			cur, prev := byMonths[months], byMonths[months-3]
			if cur == nil || prev == nil {
				continue
			}
			end := yk.start.AddDate(0, months, -1)
			q := stockKey{yk.cnpj, end}
			if quarters[q] == nil {
				quarters[q] = map[string]float64{}
			}
			for name, v := range cur {
				p, ok := prev[name]
				if _, direct := quarters[q][name]; ok && !direct {
					quarters[q][name] = v - p
				}
			}
		}
	}
	for q, m := range quarters {
		for name, v := range m {
			emit(q.cnpj, q.end, domain.PeriodQuarter, name, v)
		}
	}
	return f
}

func sumOf(acc map[string]float64, codes ...string) (float64, bool) {
	sum, any := 0.0, false
	for _, c := range codes {
		if v, ok := acc[c]; ok {
			sum += v
			any = true
		}
	}
	return sum, any
}

func monthsBetween(start, end time.Time) int {
	return (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month()) + 1
}

// mapped reports whether a statement account is used by reduce; everything
// else is dropped while reading to keep memory small.
func mapped(code string) bool {
	switch code {
	case "1", "1.01.01", "1.01.02", "2.01.04", "2.02.01", "2.03", "2.03.09",
		"3.01", "3.05", "3.11", "3.11.01", "6.01":
		return true
	}
	return strings.HasPrefix(code, "6.01.01.") || strings.HasPrefix(code, "6.02.") || strings.HasPrefix(code, "6.03.")
}

type rowFunc func(get func(string) string) (line, bool, error)

// statementRow reads a BPA/BPP/DRE/DFC row:
// CNPJ_CIA;DT_REFER;VERSAO;DENOM_CIA;CD_CVM;GRUPO_DFP;MOEDA;ESCALA_MOEDA;
// ORDEM_EXERC;DT_INI_EXERC;DT_FIM_EXERC;CD_CONTA;DS_CONTA;VL_CONTA;ST_CONTA_FIXA
func statementRow(get func(string) string) (line, bool, error) {
	// Each filing repeats the previous period ("PENÚLTIMO") for comparison.
	// The comparison is in Latin-1, so match the ASCII prefix only.
	if strings.HasPrefix(strings.ToUpper(get("ORDEM_EXERC")), "PEN") {
		return line{}, false, nil
	}
	code := get("CD_CONTA")
	if !mapped(code) {
		return line{}, false, nil
	}
	l := line{cnpj: get("CNPJ_CIA"), refer: get("DT_REFER"), code: code, desc: get("DS_CONTA")}
	var err error
	if l.version, err = strconv.Atoi(get("VERSAO")); err != nil {
		return line{}, false, fmt.Errorf("VERSAO: %w", err)
	}
	if l.end, err = time.Parse(time.DateOnly, get("DT_FIM_EXERC")); err != nil {
		return line{}, false, fmt.Errorf("DT_FIM_EXERC: %w", err)
	}
	if s := get("DT_INI_EXERC"); s != "" {
		if l.start, err = time.Parse(time.DateOnly, s); err != nil {
			return line{}, false, fmt.Errorf("DT_INI_EXERC: %w", err)
		}
	}
	if l.value, err = strconv.ParseFloat(get("VL_CONTA"), 64); err != nil {
		return line{}, false, fmt.Errorf("VL_CONTA: %w", err)
	}
	if strings.EqualFold(get("ESCALA_MOEDA"), "MIL") {
		l.value *= 1000
	}
	return l, true, nil
}

// capitalRow reads composicao_capital: total shares minus treasury shares.
func capitalRow(get func(string) string) (line, bool, error) {
	total, err := strconv.ParseFloat(get("QT_ACAO_TOTAL_CAP_INTEGR"), 64)
	if err != nil {
		return line{}, false, fmt.Errorf("QT_ACAO_TOTAL_CAP_INTEGR: %w", err)
	}
	treasury, _ := strconv.ParseFloat(get("QT_ACAO_TOTAL_TESOURO"), 64)
	l := line{cnpj: get("CNPJ_CIA"), refer: get("DT_REFER"), code: "shares", value: total - treasury}
	if l.version, err = strconv.Atoi(get("VERSAO")); err != nil {
		return line{}, false, fmt.Errorf("VERSAO: %w", err)
	}
	if l.end, err = time.Parse(time.DateOnly, l.refer); err != nil {
		return line{}, false, fmt.Errorf("DT_REFER: %w", err)
	}
	return l, true, nil
}

func readCSV(zf *zip.File, stmt string, row rowFunc) ([]line, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	r := csv.NewReader(rc)
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.TrimPrefix(strings.TrimSpace(h), "\xef\xbb\xbf")] = i
	}
	var rec []string
	get := func(name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	var out []line
	for n := 2; ; n++ {
		rec, err = r.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", n, err)
		}
		l, ok, err := row(get)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", n, err)
		}
		if ok {
			l.stmt = stmt
			out = append(out, l)
		}
	}
}

// parseTickers reads the FCA securities file and maps trading codes
// (Codigo_Negociacao) to the issuer's CNPJ.
func parseTickers(body []byte, into map[domain.Symbol]string) error {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	for _, zf := range zr.File {
		if !strings.Contains(zf.Name, "_valor_mobiliario_") {
			continue
		}
		_, err := readCSV(zf, "fca", func(get func(string) string) (line, bool, error) {
			code, cnpj := get("Codigo_Negociacao"), get("CNPJ_Companhia")
			if sym, err := domain.NewSymbol(code); err == nil && cnpj != "" {
				into[sym] = cnpj
			}
			return line{}, false, nil
		})
		if err != nil {
			return fmt.Errorf("%s: %w", zf.Name, err)
		}
		return nil
	}
	return fmt.Errorf("no valor_mobiliario file in FCA zip")
}
