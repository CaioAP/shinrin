package domain

import "time"

// PriceBar is one day of trading for an asset.
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
type CorporateAction struct {
	Asset  AssetKey
	ExDate time.Time
	Type   CorporateActionType
	Value  float64
	Source string
}

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
