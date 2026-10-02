// Package tesouro reads Tesouro Direto (Brazilian government bonds sold to
// individuals) daily rates and prices from Tesouro Transparente's open data
// CSV: the full history of every title, free, no key. It implements
// port.BondSource.
//
// Each title becomes a gov_bond asset keyed by the Treasury's short code and
// the maturity, e.g. NTNBP-20350515 for Tesouro IPCA+ 2035.
//
// Dataset: https://www.tesourotransparente.gov.br/ckan/dataset/taxas-dos-titulos-ofertados-pelo-tesouro-direto
package tesouro

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultURL is the CSV with every title's daily rates and prices.
const DefaultURL = "https://www.tesourotransparente.gov.br/ckan/dataset/df56aa42-484a-4a59-8184-7676580c81e3/resource/796d2059-14e9-44e3-80c9-2d9e30b405c1/download/PrecoTaxaTesouroDireto.csv"

const sourceName = "tesouro_transparente"

// Source downloads the Tesouro Direto CSV.
type Source struct {
	http *http.Client
	url  string
}

var _ port.BondSource = (*Source)(nil)

// New builds a source. url may be empty for DefaultURL.
func New(client *http.Client, url string) *Source {
	if url == "" {
		url = DefaultURL
	}
	return &Source{http: client, url: url}
}

// Name implements port.BondSource.
func (s *Source) Name() string { return sourceName }

// title types and their short codes (the Treasury's own bond names).
var codes = []struct{ prefix, code string }{
	// Longest prefixes first: "Tesouro IPCA+ com Juros Semestrais" before
	// "Tesouro IPCA+".
	{"Tesouro IPCA+ com Juros Semestrais", "NTNB"},
	{"Tesouro IGPM+ com Juros Semestrais", "NTNC"},
	{"Tesouro Prefixado com Juros Semestrais", "NTNF"},
	{"Tesouro Renda+ Aposentadoria Extra", "RENDA"},
	{"Tesouro Educa+", "EDUCA"},
	{"Tesouro IPCA+", "NTNBP"},
	{"Tesouro Prefixado", "LTN"},
	{"Tesouro Selic", "LFT"},
}

// BondQuotes implements port.BondSource.
func (s *Source) BondQuotes(ctx context.Context, since time.Time) ([]domain.BondQuote, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tesouro csv: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tesouro csv: status %s", resp.Status)
	}
	return parse(io.LimitReader(resp.Body, 256<<20), since)
}

// parse reads "Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;
// Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha", with
// dd/mm/yyyy dates and decimal commas.
func parse(r io.Reader, since time.Time) ([]domain.BondQuote, error) {
	cr := csv.NewReader(charmap.ISO8859_1.NewDecoder().Reader(r))
	cr.Comma, cr.LazyQuotes, cr.FieldsPerRecord = ';', true, -1
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.TrimSpace(h)] = i
	}
	for _, need := range []string{"Tipo Titulo", "Data Vencimento", "Data Base", "Taxa Compra Manha", "Taxa Venda Manha", "PU Compra Manha", "PU Venda Manha"} {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("%w: missing column %q", domain.ErrInvalid, need)
		}
	}

	var out []domain.BondQuote
	for line := 2; ; line++ {
		rec, err := cr.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		get := func(name string) string {
			if i := col[name]; i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		day, err := time.Parse("02/01/2006", get("Data Base"))
		if err != nil {
			return nil, fmt.Errorf("line %d: Data Base: %w", line, err)
		}
		if day.Before(since) {
			continue
		}
		title := get("Tipo Titulo")
		code := ""
		for _, c := range codes {
			if strings.HasPrefix(title, c.prefix) {
				code = c.code
				break
			}
		}
		if code == "" {
			continue // a title type this list does not know yet
		}
		maturity, err := time.Parse("02/01/2006", get("Data Vencimento"))
		if err != nil {
			return nil, fmt.Errorf("line %d: Data Vencimento: %w", line, err)
		}
		var nums [4]float64
		for i, name := range []string{"Taxa Compra Manha", "Taxa Venda Manha", "PU Compra Manha", "PU Venda Manha"} {
			if nums[i], err = brNumber(get(name)); err != nil {
				return nil, fmt.Errorf("line %d: %s: %w", line, name, err)
			}
		}
		sym, err := domain.NewSymbol(code + "-" + maturity.Format("20060102"))
		if err != nil {
			return nil, err
		}
		out = append(out, domain.BondQuote{
			// Market B3 stands for Brazil here: Tesouro Direto trades through
			// B3's custody, and the domain has no separate jurisdiction yet.
			Asset:    domain.AssetKey{Market: domain.MarketB3, Symbol: sym},
			Name:     fmt.Sprintf("%s %d", title, maturity.Year()),
			Maturity: maturity, Date: day,
			BuyRate: nums[0], SellRate: nums[1], BuyPrice: nums[2], SellPrice: nums[3],
			Source: sourceName,
		})
	}
}

// brNumber parses "1.234,56"; an empty field (a title no longer sold has no
// buy rate) is zero.
func brNumber(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(s, ".", ""), ",", "."), 64)
}
