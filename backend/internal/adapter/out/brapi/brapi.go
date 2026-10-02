// Package brapi reads delayed B3 quotes from brapi.dev's free plan (15,000
// requests a month, quotes refreshed about every 30 minutes, personal use).
// It implements port.QuoteSource.
//
// The free plan answers one ticker per request, so BatchSize defaults to 1;
// a paid plan can raise it. Data terms forbid republishing raw quotes, so
// Shinrin uses them for its own analysis and credits the source.
//
// API: https://brapi.dev/docs
package brapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultBaseURL is brapi's API root.
const DefaultBaseURL = "https://brapi.dev/api"

const sourceName = "brapi"

// Client calls brapi.
type Client struct {
	http      *http.Client
	baseURL   string
	token     string
	batchSize int
}

var _ port.QuoteSource = (*Client)(nil)

// New builds a client. token is the user's brapi token; batchSize is how
// many tickers one request may carry (1 on the free plan).
func New(client *http.Client, baseURL, token string, batchSize int) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{http: client, baseURL: strings.TrimSuffix(baseURL, "/"), token: token, batchSize: max(1, batchSize)}
}

// Name implements port.QuoteSource.
func (c *Client) Name() string { return sourceName }

type quoteResponse struct {
	Results []struct {
		Symbol    string  `json:"symbol"`
		Price     float64 `json:"regularMarketPrice"`
		ChangePct float64 `json:"regularMarketChangePercent"`
		Time      string  `json:"regularMarketTime"`
	} `json:"results"`
}

// Quotes implements port.QuoteSource. Tickers brapi does not know are left
// out of the result.
func (c *Client) Quotes(ctx context.Context, assets []domain.AssetKey) ([]domain.Quote, error) {
	var b3 []domain.AssetKey
	for _, a := range assets {
		if a.Market == domain.MarketB3 {
			b3 = append(b3, a)
		}
	}
	var out []domain.Quote
	for start := 0; start < len(b3); start += c.batchSize {
		batch := b3[start:min(start+c.batchSize, len(b3))]
		syms := make([]string, len(batch))
		for i, a := range batch {
			syms[i] = string(a.Symbol)
		}
		var resp quoteResponse
		found, err := c.get(ctx, "/quote/"+strings.Join(syms, ","), &resp)
		if err != nil {
			return out, fmt.Errorf("quote %s: %w", strings.Join(syms, ","), err)
		}
		if !found {
			continue
		}
		for _, r := range resp.Results {
			sym, err := domain.NewSymbol(r.Symbol)
			if err != nil || r.Price == 0 {
				continue
			}
			asOf, err := time.Parse(time.RFC3339, r.Time)
			if err != nil {
				return out, fmt.Errorf("quote %s: time %q: %w", sym, r.Time, err)
			}
			out = append(out, domain.Quote{
				Asset: domain.AssetKey{Market: domain.MarketB3, Symbol: sym},
				Price: r.Price, ChangePct: r.ChangePct, AsOf: asOf.UTC(), Source: sourceName,
			})
		}
	}
	return out, nil
}

// get returns found=false for brapi's 404 on unknown tickers.
func (c *Client) get(ctx context.Context, path string, out any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("status %s", resp.Status)
	}
	return true, json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
}
