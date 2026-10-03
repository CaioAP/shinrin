// Package tiingo reads US end-of-day prices from Tiingo's free tier: raw OHLCV
// plus the dividend and split on each day. It implements port.PriceSource and
// port.CorporateActionSource from the same endpoint.
//
// The free tier is small (at the time of writing about 50 requests per hour,
// 1,000 per day and 500 distinct tickers per month), so the rate is set in
// wiring and each price response is kept briefly to answer the corporate
// action call for the same ticker and window without a second request.
//
// API: https://www.tiingo.com/documentation/end-of-day
package tiingo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultBaseURL is Tiingo's API host.
const DefaultBaseURL = "https://api.tiingo.com"

const (
	sourceName = "tiingo"
	memoTTL    = 15 * time.Minute
)

// Client calls Tiingo's daily prices endpoint.
type Client struct {
	http    *http.Client
	baseURL string
	token   string
	now     func() time.Time

	mu   sync.Mutex
	memo map[memoKey]memoEntry
}

type memoKey struct {
	ticker string
	from   string
}

type memoEntry struct {
	rows []row
	at   time.Time
}

var (
	_ port.PriceSource           = (*Client)(nil)
	_ port.CorporateActionSource = (*Client)(nil)
)

// New builds a client. token is the user's Tiingo API token.
func New(client *http.Client, baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{http: client, baseURL: strings.TrimSuffix(baseURL, "/"), token: token, now: time.Now, memo: map[memoKey]memoEntry{}}
}

// Name implements the source ports.
func (c *Client) Name() string { return sourceName }

type row struct {
	Date        string  `json:"date"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume"`
	AdjClose    float64 `json:"adjClose"`
	DivCash     float64 `json:"divCash"`
	SplitFactor float64 `json:"splitFactor"`
}

// DailyPrices implements port.PriceSource.
func (c *Client) DailyPrices(ctx context.Context, asset domain.AssetKey, from, to time.Time) ([]domain.PriceBar, error) {
	rows, err := c.rows(ctx, asset, from)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PriceBar, 0, len(rows))
	for _, r := range rows {
		d, err := parseDate(r.Date)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", asset, err)
		}
		if d.After(to) {
			continue
		}
		out = append(out, domain.PriceBar{
			Asset: asset, Date: d,
			Open: r.Open, High: r.High, Low: r.Low, Close: r.Close,
			AdjClose: r.AdjClose, Volume: int64(r.Volume), Source: sourceName,
		})
	}
	return out, nil
}

// CorporateActions implements port.CorporateActionSource from the divCash
// and splitFactor columns.
func (c *Client) CorporateActions(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.CorporateAction, error) {
	rows, err := c.rows(ctx, asset, since)
	if err != nil {
		return nil, err
	}
	var out []domain.CorporateAction
	for _, r := range rows {
		d, err := parseDate(r.Date)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", asset, err)
		}
		if r.DivCash > 0 {
			out = append(out, domain.CorporateAction{Asset: asset, ExDate: d, Type: domain.ActionDividend, Value: r.DivCash, Source: sourceName})
		}
		if r.SplitFactor > 0 && r.SplitFactor != 1 {
			out = append(out, domain.CorporateAction{Asset: asset, ExDate: d, Type: domain.ActionSplit, Value: r.SplitFactor, Source: sourceName})
		}
	}
	return out, nil
}

// rows fetches from `from` to today, or reuses a fresh response for the same
// ticker and start date.
func (c *Client) rows(ctx context.Context, asset domain.AssetKey, from time.Time) ([]row, error) {
	if asset.Market != domain.MarketUS {
		return nil, fmt.Errorf("%s: %w", asset, domain.ErrNotFound)
	}
	// Tiingo writes share classes with a dash (BRK-B).
	ticker := strings.ToLower(strings.ReplaceAll(string(asset.Symbol), ".", "-"))
	key := memoKey{ticker, from.Format(time.DateOnly)}

	c.mu.Lock()
	if e, ok := c.memo[key]; ok && c.now().Sub(e.at) < memoTTL {
		c.mu.Unlock()
		return e.rows, nil
	}
	c.mu.Unlock()

	q := url.Values{"startDate": {key.from}, "format": {"json"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/tiingo/daily/%s/prices?%s", c.baseURL, url.PathEscape(ticker), q.Encode()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiingo %s: %w", ticker, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("tiingo %s: %w", ticker, domain.ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		// Never include the request URL or headers: they carry the token.
		return nil, fmt.Errorf("tiingo %s: status %s", ticker, resp.Status)
	}
	var rows []row
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&rows); err != nil {
		return nil, fmt.Errorf("tiingo %s: decode: %w", ticker, err)
	}

	c.mu.Lock()
	now := c.now()
	for k, e := range c.memo { // keep the memo from growing over a 500-ticker run
		if now.Sub(e.at) >= memoTTL {
			delete(c.memo, k)
		}
	}
	c.memo[key] = memoEntry{rows: rows, at: now}
	c.mu.Unlock()
	return rows, nil
}

func parseDate(s string) (time.Time, error) {
	if len(s) < 10 {
		return time.Time{}, fmt.Errorf("bad date %q", s)
	}
	return time.Parse(time.DateOnly, s[:10])
}
