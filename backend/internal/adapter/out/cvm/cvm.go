// Package cvm reads Brazilian listed companies' financial statements from
// CVM's open data portal (Dados Abertos): DFP (annual) and ITR (quarterly)
// filings, plus the FCA registration form to map tickers to companies. Free,
// no key. It implements port.FundamentalsSource.
//
// Each year's DFP or ITR is one zip holding every company's statements as
// semicolon-separated Latin-1 CSV, so the adapter downloads a year once,
// keeps only the accounts it maps, and serves every ticker from that cache.
//
// Portal: https://dados.cvm.gov.br/dataset/cia_aberta-doc-dfp
package cvm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultBaseURL is CVM's open data root.
const DefaultBaseURL = "https://dados.cvm.gov.br/dados/CIA_ABERTA/DOC"

const (
	sourceName = "cvm"
	cacheTTL   = 12 * time.Hour
)

// Client downloads and caches CVM filings.
type Client struct {
	http    *http.Client
	baseURL string
	now     func() time.Time

	mu      sync.Mutex
	filings map[string]cached // "DFP/2024" -> parsed year
	tickers map[domain.Symbol]string
	tickAt  time.Time
}

type cached struct {
	f  *filing // nil: not published (404), retried after cacheTTL
	at time.Time
}

var (
	_ port.FundamentalsSource = (*Client)(nil)
	_ port.NewsSource         = (*Client)(nil)
)

// New builds a client. baseURL may be empty for DefaultBaseURL; now may be
// nil for time.Now.
func New(client *http.Client, baseURL string, now func() time.Time) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if now == nil {
		now = time.Now
	}
	return &Client{http: client, baseURL: strings.TrimSuffix(baseURL, "/"), now: now, filings: map[string]cached{}}
}

// Name implements port.FundamentalsSource.
func (c *Client) Name() string { return sourceName }

// Fundamentals implements port.FundamentalsSource.
func (c *Client) Fundamentals(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.Fundamental, error) {
	if asset.Market != domain.MarketB3 {
		return nil, fmt.Errorf("%s: %w", asset, domain.ErrNotFound)
	}
	cnpj, err := c.cnpj(ctx, asset.Symbol)
	if err != nil {
		return nil, err
	}

	var out []domain.Fundamental
	for year := since.Year(); year <= c.now().Year(); year++ {
		for _, doc := range []string{"DFP", "ITR"} {
			f, err := c.filing(ctx, doc, year)
			if errors.Is(err, domain.ErrNotFound) {
				continue // this year's DFP is not out until the next March
			}
			if err != nil {
				return nil, err
			}
			if f.financial[cnpj] {
				// Banks and insurers file a different chart of accounts
				// (no revenue, EBIT or gross debt in the industrial sense).
				return nil, fmt.Errorf("%s is a financial institution, not mapped yet: %w", asset, domain.ErrNotFound)
			}
			for _, v := range f.values[cnpj] {
				if v.end.Before(since) {
					continue
				}
				out = append(out, domain.Fundamental{
					Asset: asset, PeriodEnd: v.end, PeriodType: v.typ,
					Metric: v.metric, Value: v.value, Source: sourceName,
				})
			}
		}
	}
	return out, nil
}

// filing returns a parsed year of DFP or ITR, downloading it at most once
// per cacheTTL.
func (c *Client) filing(ctx context.Context, doc string, year int) (*filing, error) {
	key := fmt.Sprintf("%s/%d", doc, year)
	c.mu.Lock()
	defer c.mu.Unlock() // serialises downloads too: two goroutines must not fetch the same 40 MB zip
	if e, ok := c.filings[key]; ok && c.now().Sub(e.at) < cacheTTL {
		if e.f == nil {
			return nil, fmt.Errorf("%s %d: %w", doc, year, domain.ErrNotFound)
		}
		return e.f, nil
	}
	lower := strings.ToLower(doc)
	body, err := c.download(ctx, fmt.Sprintf("%s/%s/DADOS/%s_cia_aberta_%d.zip", c.baseURL, doc, lower, year))
	if errors.Is(err, domain.ErrNotFound) {
		c.filings[key] = cached{nil, c.now()}
	}
	if err != nil {
		return nil, fmt.Errorf("%s %d: %w", doc, year, err)
	}
	periodType := domain.PeriodQuarter
	if doc == "DFP" {
		periodType = domain.PeriodFiscalYear
	}
	f, err := parseFiling(body, periodType)
	if err != nil {
		return nil, fmt.Errorf("%s %d: %w", doc, year, err)
	}
	c.filings[key] = cached{f, c.now()}
	return f, nil
}

// cnpj maps a ticker to its issuer's CNPJ.
func (c *Client) cnpj(ctx context.Context, sym domain.Symbol) (string, error) {
	tickers, err := c.tickerMap(ctx)
	if err != nil {
		return "", err
	}
	cnpj, ok := tickers[sym]
	if !ok {
		return "", fmt.Errorf("no CVM company for %s: %w", sym, domain.ErrNotFound)
	}
	return cnpj, nil
}

// tickerMap returns ticker -> CNPJ from the trading codes declared in the
// FCA of this year and the last (early in a year few FCAs are filed). The
// map is replaced, never mutated, so callers may read it without the lock.
func (c *Client) tickerMap(ctx context.Context) (map[domain.Symbol]string, error) {
	c.mu.Lock()
	if c.tickers != nil && c.now().Sub(c.tickAt) < cacheTTL {
		defer c.mu.Unlock()
		return c.tickers, nil
	}
	c.mu.Unlock()

	tickers := map[domain.Symbol]string{}
	year := c.now().Year()
	for _, y := range []int{year - 1, year} { // later year overwrites
		body, err := c.download(ctx, fmt.Sprintf("%s/FCA/DADOS/fca_cia_aberta_%d.zip", c.baseURL, y))
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("FCA %d: %w", y, err)
		}
		if err := parseTickers(body, tickers); err != nil {
			return nil, fmt.Errorf("FCA %d: %w", y, err)
		}
	}
	c.mu.Lock()
	c.tickers, c.tickAt = tickers, c.now()
	c.mu.Unlock()
	return tickers, nil
}

func (c *Client) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 512<<20))
}
