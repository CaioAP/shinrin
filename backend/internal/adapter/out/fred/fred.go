// Package fred reads US macro series from the St. Louis Fed's FRED API (free
// with a key): the federal funds rate, Treasury yields and CPI. It
// implements port.MacroSource.
//
// API: https://fred.stlouisfed.org/docs/api/fred/
package fred

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultBaseURL is FRED's API root.
const DefaultBaseURL = "https://api.stlouisfed.org/fred"

const sourceName = "fred"

var ids = map[string]string{
	domain.SeriesFedFunds: "DFF",
	domain.SeriesUST3M:    "DGS3MO",
	domain.SeriesUST2Y:    "DGS2",
	domain.SeriesUST10Y:   "DGS10",
	domain.SeriesUSCPI:    "CPIAUCSL",
}

// Client calls FRED.
type Client struct {
	http    *http.Client
	baseURL string
	key     string
}

var _ port.MacroSource = (*Client)(nil)

// New builds a client. key is the user's FRED API key.
func New(client *http.Client, baseURL, key string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{http: client, baseURL: strings.TrimSuffix(baseURL, "/"), key: key}
}

// Name implements port.MacroSource.
func (c *Client) Name() string { return sourceName }

// Serves implements port.MacroSource.
func (c *Client) Serves() []string {
	return []string{domain.SeriesFedFunds, domain.SeriesUST3M, domain.SeriesUST2Y, domain.SeriesUST10Y, domain.SeriesUSCPI}
}

// Series implements port.MacroSource.
func (c *Client) Series(ctx context.Context, series string, since time.Time) ([]domain.MacroPoint, error) {
	id, ok := ids[series]
	if !ok {
		return nil, fmt.Errorf("series %q: %w", series, domain.ErrNotFound)
	}
	q := url.Values{"series_id": {id}, "api_key": {c.key}, "file_type": {"json"}, "observation_start": {since.Format(time.DateOnly)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/series/observations?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// The URL carries the key, and *url.Error prints it; drop it.
		return nil, fmt.Errorf("fred %s: request failed: %w", id, unwrapURL(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fred %s: status %s", id, resp.Status)
	}
	var body struct {
		Observations []struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"observations"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("fred %s: decode: %w", id, err)
	}
	out := make([]domain.MacroPoint, 0, len(body.Observations))
	for _, o := range body.Observations {
		if o.Value == "." {
			continue // FRED's marker for a holiday or missing observation
		}
		d, err := time.Parse(time.DateOnly, o.Date)
		if err != nil {
			return nil, fmt.Errorf("fred %s: date %q: %w", id, o.Date, err)
		}
		v, err := strconv.ParseFloat(o.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("fred %s: value %q: %w", id, o.Value, err)
		}
		out = append(out, domain.MacroPoint{Series: series, Date: d, Value: v, Source: sourceName})
	}
	return out, nil
}

func unwrapURL(err error) error {
	if ue, ok := err.(*url.Error); ok {
		return ue.Err
	}
	return err
}
