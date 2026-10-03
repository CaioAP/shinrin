// Package sp500 lists the S&P 500's members. S&P Dow Jones Indices does not
// publish constituents for free, so this reads the community-maintained CSV
// from the datasets/s-and-p-500-companies project, which tracks Wikipedia's
// list and includes each company's SEC CIK and GICS sector. It implements
// port.UniverseSource.
//
// Source: https://github.com/datasets/s-and-p-500-companies
package sp500

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultURL is the raw CSV on GitHub.
const DefaultURL = "https://raw.githubusercontent.com/datasets/s-and-p-500-companies/main/data/constituents.csv"

// Source reads the constituents CSV.
type Source struct {
	http *http.Client
	url  string
}

var _ port.UniverseSource = (*Source)(nil)

// New builds a source. url may be empty for DefaultURL.
func New(client *http.Client, url string) *Source {
	if url == "" {
		url = DefaultURL
	}
	return &Source{http: client, url: url}
}

// Name implements port.UniverseSource.
func (s *Source) Name() string { return "sp500_datasets" }

// Constituents implements port.UniverseSource for the S&P 500.
func (s *Source) Constituents(ctx context.Context, idx domain.Index) ([]domain.Asset, error) {
	if idx != domain.IndexSP500 {
		return nil, fmt.Errorf("%w: %s is not the S&P 500", domain.ErrInvalid, idx)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get constituents: status %s", resp.Status)
	}
	return parse(io.LimitReader(resp.Body, 8<<20))
}

func parse(r io.Reader) ([]domain.Asset, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read constituents: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("constituents: %w: no rows", domain.ErrInvalid)
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.TrimSpace(h)] = i
	}
	for _, need := range []string{"Symbol", "Security", "GICS Sector"} {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("constituents: %w: missing column %q", domain.ErrInvalid, need)
		}
	}
	get := func(rec []string, name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	out := make([]domain.Asset, 0, len(rows)-1)
	for _, rec := range rows[1:] {
		sym, err := domain.NewSymbol(get(rec, "Symbol"))
		if err != nil {
			return nil, fmt.Errorf("constituents: %w", err)
		}
		class := domain.ClassStock
		if strings.Contains(get(rec, "GICS Sub-Industry"), "REIT") {
			class = domain.ClassREIT
		}
		cik := get(rec, "CIK")
		if cik != "" {
			cik = fmt.Sprintf("%010s", cik)
		}
		out = append(out, domain.Asset{
			Key:         domain.AssetKey{Market: domain.MarketUS, Symbol: sym},
			Class:       class,
			Name:        get(rec, "Security"),
			Sector:      get(rec, "GICS Sector"),
			CIK:         cik,
			IndexMember: true,
			Active:      true,
		})
	}
	return out, nil
}
