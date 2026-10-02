// Package sec reads US company fundamentals from SEC EDGAR's XBRL
// "companyfacts" API: every number a company has tagged in its 10-K and 10-Q
// filings, free, with no key. It implements port.FundamentalsSource.
//
// EDGAR requires a User-Agent naming the app and a contact email, and allows
// about 10 requests per second; both are set on the injected client (see
// internal/httpx).
//
// API: https://www.sec.gov/search-filings/edgar-application-programming-interfaces
package sec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Default endpoints.
const (
	DefaultDataURL    = "https://data.sec.gov"
	DefaultTickersURL = "https://www.sec.gov/files/company_tickers.json"
)

const sourceName = "sec_edgar"

// Client fetches companyfacts.
type Client struct {
	http       *http.Client
	dataURL    string
	tickersURL string

	mu   sync.Mutex
	ciks map[domain.Symbol]string // loaded once per process
}

var _ port.FundamentalsSource = (*Client)(nil)

// New builds a client. Empty URLs use the defaults.
func New(client *http.Client, dataURL, tickersURL string) *Client {
	if dataURL == "" {
		dataURL = DefaultDataURL
	}
	if tickersURL == "" {
		tickersURL = DefaultTickersURL
	}
	return &Client{http: client, dataURL: strings.TrimSuffix(dataURL, "/"), tickersURL: tickersURL}
}

// Name implements port.FundamentalsSource.
func (c *Client) Name() string { return sourceName }

// concept maps one domain metric to XBRL tags, in order of preference.
// Companies tag the same line differently (Apple reports revenue as
// RevenueFromContractWithCustomerExcludingAssessedTax, older filings as
// SalesRevenueNet), so the first tag with data for a period wins.
type concept struct {
	metric   string
	taxonomy string
	tags     []string
	unit     string
	instant  bool // balance-sheet value at a date rather than over a period
}

var concepts = []concept{
	{domain.MetricRevenue, "us-gaap", []string{"RevenueFromContractWithCustomerExcludingAssessedTax", "Revenues", "SalesRevenueNet", "RevenueFromContractWithCustomerIncludingAssessedTax"}, "USD", false},
	{domain.MetricEBIT, "us-gaap", []string{"OperatingIncomeLoss"}, "USD", false},
	{domain.MetricDepreciation, "us-gaap", []string{"DepreciationDepletionAndAmortization", "DepreciationAmortizationAndAccretionNet", "DepreciationAndAmortization", "Depreciation"}, "USD", false},
	{domain.MetricNetIncome, "us-gaap", []string{"NetIncomeLoss", "ProfitLoss"}, "USD", false},
	{domain.MetricOperatingCashFlow, "us-gaap", []string{"NetCashProvidedByUsedInOperatingActivities"}, "USD", false},
	{domain.MetricCapex, "us-gaap", []string{"PaymentsToAcquirePropertyPlantAndEquipment", "PaymentsToAcquireProductiveAssets"}, "USD", false},
	{domain.MetricDividendsPaid, "us-gaap", []string{"PaymentsOfDividends", "PaymentsOfDividendsCommonStock"}, "USD", false},
	{domain.MetricTotalAssets, "us-gaap", []string{"Assets"}, "USD", true},
	{domain.MetricEquity, "us-gaap", []string{"StockholdersEquity", "StockholdersEquityIncludingPortionAttributableToNoncontrollingInterest"}, "USD", true},
	{domain.MetricCash, "us-gaap", []string{"CashAndCashEquivalentsAtCarryingValue", "CashCashEquivalentsRestrictedCashAndRestrictedCashEquivalents"}, "USD", true},
	{domain.MetricShares, "dei", []string{"EntityCommonStockSharesOutstanding"}, "shares", true},
}

// Gross debt is the sum of parts companies tag separately; summed per period.
var debtParts = [][]string{
	{"LongTermDebtNoncurrent", "LongTermDebt"},
	{"LongTermDebtCurrent"},
	{"ShortTermBorrowings", "CommercialPaper"},
}

// facts is companyfacts' "facts" object: taxonomy -> tag -> units.
type facts map[string]map[string]struct {
	Units map[string][]fact `json:"units"`
}

type companyFacts struct {
	Facts facts `json:"facts"`
}

type fact struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Val   float64 `json:"val"`
	Form  string  `json:"form"`
	Filed string  `json:"filed"`
}

// Fundamentals implements port.FundamentalsSource.
func (c *Client) Fundamentals(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.Fundamental, error) {
	if asset.Market != domain.MarketUS {
		return nil, fmt.Errorf("%s: %w", asset, domain.ErrNotFound)
	}
	cik, err := c.cik(ctx, asset.Symbol)
	if err != nil {
		return nil, err
	}
	var cf companyFacts
	if err := c.getJSON(ctx, fmt.Sprintf("%s/api/xbrl/companyfacts/CIK%s.json", c.dataURL, cik), &cf); err != nil {
		return nil, fmt.Errorf("companyfacts of %s: %w", asset, err)
	}
	return mapFacts(asset, cf.Facts, since), nil
}

// mapFacts converts companyfacts to domain fundamentals.
func mapFacts(asset domain.AssetKey, fs facts, since time.Time) []domain.Fundamental {
	var out []domain.Fundamental
	emit := func(metric string, vals map[periodKey]float64) {
		for k, v := range vals {
			if k.end.Before(since) {
				continue
			}
			out = append(out, domain.Fundamental{Asset: asset, PeriodEnd: k.end, PeriodType: k.typ, Metric: metric, Value: v, Source: sourceName})
		}
	}

	for _, c := range concepts {
		merged := map[periodKey]float64{}
		for _, tag := range c.tags { // earlier tags win per period
			for k, v := range periods(fs[c.taxonomy][tag].Units[c.unit], c.instant) {
				if _, have := merged[k]; !have {
					merged[k] = v
				}
			}
		}
		emit(c.metric, merged)
	}

	debt := map[periodKey]float64{}
	for _, alternatives := range debtParts {
		part := map[periodKey]float64{}
		for _, tag := range alternatives {
			for k, v := range periods(fs["us-gaap"][tag].Units["USD"], true) {
				if _, have := part[k]; !have {
					part[k] = v
				}
			}
		}
		for k, v := range part {
			debt[k] += v
		}
	}
	emit(domain.MetricGrossDebt, debt)
	return out
}

type periodKey struct {
	end time.Time
	typ domain.PeriodType
}

// periods keeps one value per period: quarters (about 91 days) and fiscal
// years (about 365 days) for flows; balances are typed by the form that
// reported them (annual or quarterly). Six- and nine-month year-to-date
// values are dropped; the domain derives the fourth quarter from the year.
// When a value is restated, the latest filing wins.
func periods(facts []fact, instant bool) map[periodKey]float64 {
	type pick struct {
		v     float64
		filed string
	}
	best := map[periodKey]pick{}
	for _, f := range facts {
		if !annual(f.Form) && !strings.HasPrefix(f.Form, "10-Q") {
			continue // 8-Ks and registration statements repeat numbers unaudited
		}
		end, err := time.Parse(time.DateOnly, f.End)
		if err != nil {
			continue
		}
		var typ domain.PeriodType
		if instant {
			typ = domain.PeriodQuarter
			if annual(f.Form) {
				typ = domain.PeriodFiscalYear
			}
		} else {
			start, err := time.Parse(time.DateOnly, f.Start)
			if err != nil {
				continue
			}
			switch days := end.Sub(start).Hours() / 24; {
			case days >= 80 && days <= 100:
				typ = domain.PeriodQuarter
			case days >= 350 && days <= 380:
				typ = domain.PeriodFiscalYear
			default:
				continue
			}
		}
		k := periodKey{end, typ}
		if cur, ok := best[k]; !ok || f.Filed > cur.filed {
			best[k] = pick{f.Val, f.Filed}
		}
	}
	out := make(map[periodKey]float64, len(best))
	for k, p := range best {
		out[k] = p.v
	}
	return out
}

func annual(form string) bool {
	return strings.HasPrefix(form, "10-K") || strings.HasPrefix(form, "20-F") || strings.HasPrefix(form, "40-F")
}

// cik resolves a ticker through SEC's ticker list, loaded once.
func (c *Client) cik(ctx context.Context, sym domain.Symbol) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ciks == nil {
		var raw map[string]struct {
			CIK    json.Number `json:"cik_str"`
			Ticker string      `json:"ticker"`
		}
		if err := c.getJSON(ctx, c.tickersURL, &raw); err != nil {
			return "", fmt.Errorf("SEC ticker list: %w", err)
		}
		c.ciks = make(map[domain.Symbol]string, len(raw))
		for _, r := range raw {
			n, err := strconv.ParseInt(r.CIK.String(), 10, 64)
			if err != nil || n <= 0 {
				continue
			}
			// SEC writes class shares with a dash (BRK-B); Shinrin and most
			// index lists use a dot (BRK.B).
			sym := domain.Symbol(strings.ToUpper(strings.ReplaceAll(r.Ticker, "-", ".")))
			c.ciks[sym] = fmt.Sprintf("%010d", n)
		}
	}
	cik, ok := c.ciks[sym]
	if !ok {
		return "", fmt.Errorf("no CIK for %s: %w", sym, domain.ErrNotFound)
	}
	return cik, nil
}

func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return domain.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
	}
	// companyfacts for large filers is 10-20 MB.
	return json.NewDecoder(io.LimitReader(resp.Body, 128<<20)).Decode(out)
}
