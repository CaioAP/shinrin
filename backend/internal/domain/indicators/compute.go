package indicators

import (
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Indicator names, used as keys in domain.IndicatorSet.Values. Ratios are
// fractions (0.12 means 12%), money amounts are in the asset's currency.
const (
	Close         = "close"
	SMA50         = "sma_50"
	SMA200        = "sma_200"
	RSI14         = "rsi_14"
	MACDLine      = "macd"
	MACDSignal    = "macd_signal"
	MACDHistogram = "macd_hist"
	Return1M      = "return_1m"
	Return3M      = "return_3m"
	Return6M      = "return_6m"
	Return12M     = "return_12m"
	Volatility1Y  = "volatility_1y"
	MaxDrawdown1Y = "max_drawdown_1y"
	High52W       = "high_52w"
	Low52W        = "low_52w"

	MarketCap       = "market_cap"
	EnterpriseValue = "enterprise_value"
	PE              = "pe"
	PB              = "pb"
	PriceToSales    = "ps"
	EVToEBITDA      = "ev_ebitda"
	EVToEBIT        = "ev_ebit"
	DividendYield   = "dividend_yield"
	FCFYield        = "fcf_yield"
	ROE             = "roe"
	NetMargin       = "net_margin"
	EBITMargin      = "ebit_margin"
	NetDebtToEBITDA = "net_debt_ebitda"

	RevenueGrowth1Y  = "revenue_growth_1y"
	EarningsGrowth1Y = "earnings_growth_1y"
	RevenueCAGR3Y    = "revenue_cagr_3y"
)

// Trading-day windows for the return indicators.
const (
	month    = 21
	quarter  = 63
	halfYear = 126
	year     = TradingDaysPerYear
)

// Technicals computes every price-based indicator from adjusted closes, oldest
// first.
func Technicals(closes []float64) map[string]float64 {
	out := make(map[string]float64)
	if len(closes) == 0 {
		return out
	}
	out[Close] = closes[len(closes)-1]
	// put(name)(f()) stores f's value only when f reports ok.
	put := func(name string) func(float64, bool) {
		return func(v float64, ok bool) {
			if ok {
				out[name] = v
			}
		}
	}
	put(SMA50)(SMA(closes, 50))
	put(SMA200)(SMA(closes, 200))
	put(RSI14)(RSI(closes, 14))
	if m, ok := MACD(closes, 12, 26, 9); ok {
		out[MACDLine], out[MACDSignal], out[MACDHistogram] = m.MACD, m.Signal, m.Histogram
	}
	put(Return1M)(Return(closes, month))
	put(Return3M)(Return(closes, quarter))
	put(Return6M)(Return(closes, halfYear))
	put(Return12M)(Return(closes, year))
	put(Volatility1Y)(Volatility(closes, min(year, len(closes)-1)))
	window := closes[max(0, len(closes)-year-1):]
	put(MaxDrawdown1Y)(MaxDrawdown(window))
	if h, l, ok := HighLow(closes, min(year, len(closes))); ok {
		out[High52W], out[Low52W] = h, l
	}
	return out
}

// Snapshot computes technicals and valuation for one asset as of the last bar.
//
// Closes are always back-adjusted here from raw closes and the stored
// corporate actions, even when a source publishes its own adjusted close: a
// vendor's adjusted history changes on every new dividend, so a stored copy
// goes stale, while raw closes never change.
func Snapshot(asset domain.AssetKey, bars []domain.PriceBar, fundamentals []domain.Fundamental, actions []domain.CorporateAction) (domain.IndicatorSet, bool) {
	if len(bars) == 0 {
		return domain.IndicatorSet{}, false
	}
	bars = domain.AdjustCloses(bars, actions)
	closes := make([]float64, 0, len(bars))
	for _, b := range bars {
		closes = append(closes, b.AdjClose)
	}
	last := bars[len(bars)-1]
	values := Technicals(closes)
	// Valuation uses the traded price, not the back-adjusted one (they are
	// equal on the last bar, but be explicit).
	values[Close] = last.Close
	fin := domain.NewFinancials(fundamentals)
	for k, v := range Valuation(last.Close, FundamentalsAt(fin, actions, last.Date)) {
		values[k] = v
	}
	for k, v := range Growth(fin, last.Date) {
		values[k] = v
	}
	return domain.IndicatorSet{Asset: asset, AsOf: last.Date, Values: values}, true
}
