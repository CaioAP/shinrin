// Package finnhub reads near-real-time US quotes and company news from
// Finnhub's free tier (about 60 calls a minute, personal use). It implements
// port.QuoteSource and port.CompanyNewsSource. Historical candles are a paid
// feature, which is why daily bars come from Tiingo instead.
//
// API: https://finnhub.io/docs/api
package finnhub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultBaseURL is Finnhub's API root.
const DefaultBaseURL = "https://finnhub.io/api/v1"

const sourceName = "finnhub"

// Client calls Finnhub.
type Client struct {
	http    *http.Client
	baseURL string
	token   string
	now     func() time.Time
}

var (
	_ port.QuoteSource       = (*Client)(nil)
	_ port.CompanyNewsSource = (*Client)(nil)
)

// New builds a client. token is the user's Finnhub API key.
func New(client *http.Client, baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{http: client, baseURL: strings.TrimSuffix(baseURL, "/"), token: token, now: time.Now}
}

// Name implements the source ports.
func (c *Client) Name() string { return sourceName }

// Quotes implements port.QuoteSource, one request per asset. Assets Finnhub
// does not know are left out of the result.
func (c *Client) Quotes(ctx context.Context, assets []domain.AssetKey) ([]domain.Quote, error) {
	var out []domain.Quote
	for _, a := range assets {
		if a.Market != domain.MarketUS {
			continue
		}
		var q struct {
			Current   float64 `json:"c"`
			ChangePct float64 `json:"dp"`
			Time      int64   `json:"t"`
		}
		if err := c.get(ctx, "/quote", url.Values{"symbol": {symbol(a)}}, &q); err != nil {
			return out, fmt.Errorf("quote %s: %w", a, err)
		}
		if q.Current == 0 || q.Time == 0 {
			continue // Finnhub answers unknown symbols with zeros
		}
		out = append(out, domain.Quote{Asset: a, Price: q.Current, ChangePct: q.ChangePct, AsOf: time.Unix(q.Time, 0).UTC(), Source: sourceName})
	}
	return out, nil
}

// CompanyNews implements port.CompanyNewsSource. Finnhub keeps about a year
// of company news on the free tier; headlines and summaries only, never full
// articles.
func (c *Client) CompanyNews(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.NewsItem, error) {
	if asset.Market != domain.MarketUS {
		return nil, fmt.Errorf("%s: %w", asset, domain.ErrNotFound)
	}
	var rows []struct {
		Datetime int64  `json:"datetime"`
		Headline string `json:"headline"`
		Source   string `json:"source"`
		Summary  string `json:"summary"`
		URL      string `json:"url"`
	}
	q := url.Values{"symbol": {symbol(asset)}, "from": {since.Format(time.DateOnly)}, "to": {c.now().Format(time.DateOnly)}}
	if err := c.get(ctx, "/company-news", q, &rows); err != nil {
		return nil, fmt.Errorf("news %s: %w", asset, err)
	}
	out := make([]domain.NewsItem, 0, len(rows))
	for _, r := range rows {
		published := time.Unix(r.Datetime, 0).UTC()
		if r.URL == "" || r.Headline == "" || published.Before(since) {
			continue
		}
		out = append(out, domain.NewsItem{
			URL: r.URL, Title: r.Headline, Summary: r.Summary, Lang: "en",
			// Keep the publisher for credit; the fetching source is Finnhub.
			Source:      sourceName + ":" + r.Source,
			PublishedAt: published,
			Assets:      []domain.AssetKey{asset},
		})
	}
	return out, nil
}

// Finnhub writes share classes with a dot, like Shinrin (BRK.B).
func symbol(a domain.AssetKey) string { return string(a.Symbol) }

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Finnhub-Token", c.token) // header, so the key never lands in a logged URL
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out)
}
