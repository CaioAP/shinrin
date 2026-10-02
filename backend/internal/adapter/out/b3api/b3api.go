// Package b3api reads two of the JSON services behind B3's public website: the
// theoretical portfolio of an index (which tickers are in the Ibovespa) and a
// listed company's corporate actions (cash dividends, JCP, splits, reverse
// splits and bonus shares). It implements port.UniverseSource and
// port.CorporateActionSource.
//
// These services are not documented as an API. They are what B3's own pages
// call, they need no key, and they are the only free source of B3 dividends
// besides parsing CVM filings, so this adapter keeps parsing strict and fails
// loudly when the shape changes.
package b3api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultBaseURL is the host of B3's listing services.
const DefaultBaseURL = "https://sistemaswebb3-listados.b3.com.br"

const sourceName = "b3_api"

// Client calls B3's services.
type Client struct {
	http    *http.Client
	baseURL string
}

var (
	_ port.UniverseSource        = (*Client)(nil)
	_ port.CorporateActionSource = (*Client)(nil)
)

// New builds a client. baseURL may be empty for DefaultBaseURL.
func New(client *http.Client, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{http: client, baseURL: strings.TrimSuffix(baseURL, "/")}
}

// Name implements the source ports.
func (c *Client) Name() string { return sourceName }

// --- Index portfolio -----------------------------------------------------------

type portfolioResponse struct {
	Page struct {
		TotalPages int `json:"totalPages"`
	} `json:"page"`
	Results []struct {
		Cod   string `json:"cod"`
		Asset string `json:"asset"`
		Type  string `json:"type"`
	} `json:"results"`
}

// Constituents implements port.UniverseSource for B3 indices (IBOV).
func (c *Client) Constituents(ctx context.Context, idx domain.Index) ([]domain.Asset, error) {
	if idx.Market() != domain.MarketB3 {
		return nil, fmt.Errorf("%w: %s is not a B3 index", domain.ErrInvalid, idx)
	}
	var out []domain.Asset
	for page := 1; ; page++ {
		var resp portfolioResponse
		q := map[string]any{"language": "pt-br", "pageNumber": page, "pageSize": 120, "index": string(idx), "segment": "1"}
		if err := c.get(ctx, "/indexProxy/indexCall/GetPortfolioDay/", q, &resp); err != nil {
			return nil, fmt.Errorf("%s portfolio page %d: %w", idx, page, err)
		}
		for _, r := range resp.Results {
			sym, err := domain.NewSymbol(r.Cod)
			if err != nil {
				return nil, fmt.Errorf("%s portfolio: %w", idx, err)
			}
			// Index members are shares or units of companies; an "11"
			// ticker here is a unit (TAEE11), never a real estate fund.
			out = append(out, domain.Asset{
				Key:         domain.AssetKey{Market: domain.MarketB3, Symbol: sym},
				Class:       domain.ClassStock,
				Name:        strings.TrimSpace(r.Asset),
				IndexMember: true,
				Active:      true,
			})
		}
		if page >= resp.Page.TotalPages {
			break
		}
	}
	return out, nil
}

// --- Corporate actions -----------------------------------------------------------

type supplementResponse []struct {
	CashDividends []struct {
		ISIN          string `json:"isinCode"`
		Rate          string `json:"rate"`
		Label         string `json:"label"`
		LastDatePrior string `json:"lastDatePrior"`
	} `json:"cashDividends"`
	StockDividends []struct {
		ISIN          string `json:"isinCode"`
		Factor        string `json:"factor"`
		Label         string `json:"label"`
		LastDatePrior string `json:"lastDatePrior"`
	} `json:"stockDividends"`
}

// CorporateActions implements port.CorporateActionSource. B3 returns the
// company's full history in one call; since only filters the result.
//
// B3 gives the last day the share traded "with" the right (lastDatePrior);
// the ex-date stored is the next calendar day, and price adjustment applies it
// at the first trading day on or after that.
func (c *Client) CorporateActions(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.CorporateAction, error) {
	if asset.Market != domain.MarketB3 || len(asset.Symbol) < 5 {
		return nil, fmt.Errorf("%s: %w", asset, domain.ErrNotFound)
	}
	issuer := string(asset.Symbol[:4])
	var resp supplementResponse
	q := map[string]any{"issuingCompany": issuer, "language": "pt-br"}
	if err := c.get(ctx, "/listedCompaniesProxy/CompanyCall/GetListedSupplementCompany/", q, &resp); err != nil {
		return nil, fmt.Errorf("corporate actions of %s: %w", issuer, err)
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("%s: %w", asset, domain.ErrNotFound)
	}

	type key struct {
		ex  time.Time
		typ domain.CorporateActionType
	}
	agg := map[key]domain.CorporateAction{}
	add := func(a domain.CorporateAction) {
		k := key{a.ExDate, a.Type}
		if cur, ok := agg[k]; ok {
			if a.IsCash() {
				a.Value += cur.Value // two dividends with the same ex-date are paid together
			} else {
				a.Value *= cur.Value
			}
		}
		agg[k] = a
	}

	for _, d := range resp[0].CashDividends {
		if !sameClass(asset.Symbol, d.ISIN) {
			continue
		}
		typ, ok := cashType(d.Label)
		if !ok {
			continue
		}
		ex, err := exDate(d.LastDatePrior)
		if err != nil {
			return nil, fmt.Errorf("%s dividend: %w", asset, err)
		}
		v, err := brNumber(d.Rate)
		if err != nil {
			return nil, fmt.Errorf("%s dividend rate: %w", asset, err)
		}
		add(domain.CorporateAction{Asset: asset, ExDate: ex, Type: typ, Value: v, Source: sourceName})
	}
	for _, d := range resp[0].StockDividends {
		if !sameClass(asset.Symbol, d.ISIN) {
			continue
		}
		ex, err := exDate(d.LastDatePrior)
		if err != nil {
			return nil, fmt.Errorf("%s stock event: %w", asset, err)
		}
		f, err := brNumber(d.Factor)
		if err != nil {
			return nil, fmt.Errorf("%s stock event factor: %w", asset, err)
		}
		a := domain.CorporateAction{Asset: asset, ExDate: ex, Source: sourceName}
		switch strings.ToUpper(strings.TrimSpace(d.Label)) {
		case "DESDOBRAMENTO": // split; factor is the % of new shares (100 = 2-for-1)
			a.Type, a.Value = domain.ActionSplit, 1+f/100
		case "BONIFICACAO", "BONIFICAÇÃO": // bonus shares; factor in %
			a.Type, a.Value = domain.ActionBonus, 1+f/100
		case "GRUPAMENTO": // reverse split; factor is shares after per share before
			a.Type, a.Value = domain.ActionSplit, f
			if f > 1 {
				a.Value = 1 / f // some events quote "10" for 10-into-1
			}
		default:
			continue
		}
		if a.Value > 0 {
			add(a)
		}
	}

	out := make([]domain.CorporateAction, 0, len(agg))
	for _, a := range agg {
		if !a.ExDate.Before(since) {
			out = append(out, a)
		}
	}
	slices.SortFunc(out, func(a, b domain.CorporateAction) int { return a.ExDate.Compare(b.ExDate) })
	return out, nil
}

func cashType(label string) (domain.CorporateActionType, bool) {
	switch l := strings.ToUpper(strings.TrimSpace(label)); {
	case strings.HasPrefix(l, "JRS CAP PROPRIO"), strings.HasPrefix(l, "JUROS"):
		return domain.ActionJCP, true
	case strings.HasPrefix(l, "DIVIDENDO"), strings.HasPrefix(l, "RENDIMENTO"):
		return domain.ActionDividend, true
	}
	return "", false
}

// sameClass reports whether an ISIN is for the share class of sym. B3 ISINs
// encode the class: BRPETRACNOR9 is ordinary (PETR3), BRPETRACNPR6 preferred
// (PETR4), BRTAEECDAM10 a unit (TAEE11).
func sameClass(sym domain.Symbol, isin string) bool {
	codes := map[string]string{"3": "ACNOR", "4": "ACNPR", "5": "ACNPA", "6": "ACNPB", "7": "ACNPC", "8": "ACNPD", "11": "CDAM"}
	code, ok := codes[string(sym[4:])]
	if !ok || len(isin) < 12 || isin[2:6] != string(sym[:4]) {
		return false
	}
	return strings.HasPrefix(isin[6:], code)
}

func exDate(lastWith string) (time.Time, error) {
	t, err := time.Parse("02/01/2006", strings.TrimSpace(lastWith))
	if err != nil {
		return time.Time{}, err
	}
	return t.AddDate(0, 0, 1), nil
}

// brNumber parses "1.234,56" style numbers.
func brNumber(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ".", "")
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
}

// get calls a B3 service, which takes its JSON parameters base64-encoded in
// the last path segment.
func (c *Client) get(ctx context.Context, path string, params any, out any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+base64.StdEncoding.EncodeToString(raw), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
