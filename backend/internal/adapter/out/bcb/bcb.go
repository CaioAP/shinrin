// Package bcb reads Brazilian macro series from the Banco Central do
// Brasil's SGS open data API (free, no key): Selic target, CDI, IPCA and the
// PTAX dollar rate. It implements port.MacroSource.
//
// Daily series reject windows longer than ten years, so requests are split
// into five-year chunks.
//
// API: https://dadosabertos.bcb.gov.br/dataset/sgs
package bcb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultBaseURL is the SGS API root.
const DefaultBaseURL = "https://api.bcb.gov.br/dados/serie"

const sourceName = "bcb_sgs"

// codes maps Shinrin series to SGS series numbers.
var codes = map[string]int{
	domain.SeriesSelicTarget: 432, // Meta Selic definida pelo Copom
	domain.SeriesCDI:         12,  // CDI, % a.d.
	domain.SeriesIPCA:        433, // IPCA, variação mensal
	domain.SeriesUSDBRL:      1,   // Dólar americano (venda), PTAX
}

// Client calls the SGS API.
type Client struct {
	http    *http.Client
	baseURL string
	now     func() time.Time
}

var _ port.MacroSource = (*Client)(nil)

// New builds a client. baseURL may be empty for DefaultBaseURL.
func New(client *http.Client, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{http: client, baseURL: strings.TrimSuffix(baseURL, "/"), now: time.Now}
}

// Name implements port.MacroSource.
func (c *Client) Name() string { return sourceName }

// Serves implements port.MacroSource.
func (c *Client) Serves() []string {
	return []string{domain.SeriesSelicTarget, domain.SeriesCDI, domain.SeriesIPCA, domain.SeriesUSDBRL}
}

// Series implements port.MacroSource.
func (c *Client) Series(ctx context.Context, series string, since time.Time) ([]domain.MacroPoint, error) {
	code, ok := codes[series]
	if !ok {
		return nil, fmt.Errorf("series %q: %w", series, domain.ErrNotFound)
	}
	var out []domain.MacroPoint
	end := c.now()
	for from := since; !from.After(end); from = from.AddDate(5, 0, 0) {
		to := from.AddDate(5, 0, -1)
		if to.After(end) {
			to = end
		}
		pts, err := c.fetch(ctx, series, code, from, to)
		if err != nil {
			return nil, err
		}
		out = append(out, pts...)
	}
	return out, nil
}

func (c *Client) fetch(ctx context.Context, series string, code int, from, to time.Time) ([]domain.MacroPoint, error) {
	url := fmt.Sprintf("%s/bcdata.sgs.%d/dados?formato=json&dataInicial=%s&dataFinal=%s",
		c.baseURL, code, from.Format("02/01/2006"), to.Format("02/01/2006"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sgs %d: %w", code, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // SGS answers 404 for a window with no observations
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sgs %d: status %s", code, resp.Status)
	}
	var rows []struct {
		Data  string `json:"data"`
		Valor string `json:"valor"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&rows); err != nil {
		return nil, fmt.Errorf("sgs %d: decode: %w", code, err)
	}
	out := make([]domain.MacroPoint, 0, len(rows))
	for _, r := range rows {
		d, err := time.Parse("02/01/2006", r.Data)
		if err != nil {
			return nil, fmt.Errorf("sgs %d: date %q: %w", code, r.Data, err)
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(r.Valor), 64)
		if err != nil {
			return nil, fmt.Errorf("sgs %d: value %q: %w", code, r.Valor, err)
		}
		out = append(out, domain.MacroPoint{Series: series, Date: d, Value: v, Source: sourceName})
	}
	return out, nil
}
