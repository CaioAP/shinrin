package domain

import "time"

// PriceBar is one day of trading for an asset. Open to Close are raw traded
// prices. AdjClose is the source's own adjusted close, or zero when it
// publishes raw prices only; analysis recomputes it with AdjustCloses.
type PriceBar struct {
	Asset    AssetKey
	Date     time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	AdjClose float64
	Volume   int64
	Source   string
}

// Quote is the latest known price, possibly delayed.
type Quote struct {
	Asset     AssetKey
	Price     float64
	ChangePct float64
	AsOf      time.Time
	Source    string
}

// CorporateActionType is the kind of event that changes holders' positions.
type CorporateActionType string

const (
	ActionDividend CorporateActionType = "dividend"
	ActionJCP      CorporateActionType = "jcp"
	ActionSplit    CorporateActionType = "split"
	ActionBonus    CorporateActionType = "bonus"
)

// CorporateAction is a dividend, JCP, split or bonus event.
//
// Value is the cash amount per share for dividends and JCP, and the share
// ratio (shares after / shares before) for splits and bonuses: 2 for a 2-for-1
// split, 1.1 for a 10% bonus, 0.1 for a 10-to-1 reverse split.
type CorporateAction struct {
	Asset  AssetKey
	ExDate time.Time
	Type   CorporateActionType
	Value  float64
	Source string
}

// IsCash reports whether the action pays cash to holders.
func (a CorporateAction) IsCash() bool { return a.Type == ActionDividend || a.Type == ActionJCP }

// PeriodType is the length of a reporting period.
type PeriodType string

const (
	PeriodQuarter    PeriodType = "Q"
	PeriodFiscalYear PeriodType = "FY"
)

// Fundamental is one reported metric (long format: revenue, net_income, ...).
type Fundamental struct {
	Asset      AssetKey
	PeriodEnd  time.Time
	PeriodType PeriodType
	Metric     string
	Value      float64
	Source     string
}

// NewsItem is an article or regulatory filing headline.
type NewsItem struct {
	URL         string
	Title       string
	Summary     string
	Lang        string
	Source      string
	PublishedAt time.Time
	Assets      []AssetKey
}

// MacroPoint is one observation of a macro series such as selic or ipca.
type MacroPoint struct {
	Series string
	Date   time.Time
	Value  float64
	Source string
}

// IndicatorSet holds the indicators computed for one asset on one date, keyed
// by name (see package domain/indicators for the names). A missing key means
// the indicator could not be computed (not enough history, negative
// earnings, ...), which is different from zero.
type IndicatorSet struct {
	Asset  AssetKey
	AsOf   time.Time
	Values map[string]float64
}

// Macro series codes stored in macro_series. Sources map their own ids (BCB
// SGS numbers, FRED series ids) onto these.
const (
	SeriesSelicTarget = "selic"     // Copom target rate, % a year
	SeriesCDI         = "cdi"       // CDI, % a day
	SeriesIPCA        = "ipca"      // IPCA monthly change, %
	SeriesUSDBRL      = "usdbrl"    // PTAX selling rate, BRL per USD
	SeriesFedFunds    = "fed_funds" // effective federal funds rate, % a year
	SeriesUST3M       = "ust_3m"    // Treasury constant maturity yields, % a year
	SeriesUST2Y       = "ust_2y"
	SeriesUST10Y      = "ust_10y"
	SeriesUSCPI       = "us_cpi" // CPI-U index level
)

// BondQuote is one day of a government bond's buy and sell rates and prices
// (Tesouro Direto). Rates are % a year; for inflation-linked bonds they are
// the real rate on top of inflation.
type BondQuote struct {
	Asset     AssetKey
	Name      string // e.g. "Tesouro IPCA+ 2035"
	Maturity  time.Time
	Date      time.Time
	BuyRate   float64
	SellRate  float64
	BuyPrice  float64
	SellPrice float64
	Source    string
}

// MacroIndicator is one headline macro number in the unit users read it in
// (CDI as % a year rather than % a day, IPCA over twelve months rather than
// one).
type MacroIndicator struct {
	Code   string // selic, cdi, ipca_12m, usdbrl, fed_funds, ust_10y, us_cpi_12m
	Value  float64
	Unit   string // "pct_year" or "brl_per_usd"
	AsOf   time.Time
	Source string
}

// MacroStrip is the dashboard's macro row. Missing lists the indicators that
// could not be computed from stored data, so the UI can say so instead of
// hiding them.
type MacroStrip struct {
	Items   []MacroIndicator
	Missing []string
}
