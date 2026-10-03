package scoring

import (
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
)

// Timing thresholds, the usual technical-analysis conventions.
const (
	RSIOverbought = 70
	RSIOversold   = 30
	// NearLow and NearHigh: within 10% above the 52-week low, or within 3%
	// of the 52-week high.
	NearLow  = 0.10
	NearHigh = -0.03
)

// TimingOf answers "when": trend, momentum and overbought or oversold
// conditions from the technicals, combined with the valuation view. The
// technicals inform timing only; whether the asset is worth owning is the
// factor scores' job. metrics must include the derived ratios (see Metrics).
func TimingOf(m map[string]float64, view domain.ValuationView) domain.Timing {
	var s []domain.Signal
	add := func(code, msg string, w float64, data map[string]float64) {
		s = append(s, domain.Signal{Code: code, Message: msg, Weight: w, Data: data})
	}

	vs200, has200 := m[PriceVsSMA200]
	sma50, has50 := m[indicators.SMA50]
	sma200, hasSMA := m[indicators.SMA200]
	if has200 && has50 && hasSMA {
		data := map[string]float64{PriceVsSMA200: round4(vs200), indicators.SMA50: round2(sma50), indicators.SMA200: round2(sma200)}
		switch {
		case vs200 > 0 && sma50 > sma200:
			add("uptrend", "Price and the 50-day average are above the 200-day average.", 1, data)
		case vs200 < 0 && sma50 < sma200:
			add("downtrend", "Price and the 50-day average are below the 200-day average.", -1, data)
		}
	}
	if h, ok := m[indicators.MACDHistogram]; ok {
		if h > 0 {
			add("macd_positive", "MACD is above its signal line (momentum improving).", 0.5, map[string]float64{indicators.MACDHistogram: round4(h)})
		} else if h < 0 {
			add("macd_negative", "MACD is below its signal line (momentum fading).", -0.5, map[string]float64{indicators.MACDHistogram: round4(h)})
		}
	}
	if rsi, ok := m[indicators.RSI14]; ok {
		data := map[string]float64{indicators.RSI14: round2(rsi)}
		switch {
		case rsi > RSIOverbought:
			add("rsi_overbought", "RSI is above 70: the recent rise looks stretched.", -1, data)
		case rsi < RSIOversold:
			add("rsi_oversold", "RSI is below 30: selling looks overdone.", 0.5, data)
		}
	}
	if fl, ok := m[FromLow52W]; ok && fl <= NearLow && view == domain.ValuationCheap {
		add("near_low_cheap", "Near the 52-week low while valuation looks cheap.", 1, map[string]float64{FromLow52W: round4(fl)})
	}
	if fh, ok := m[FromHigh52W]; ok && fh >= NearHigh && view == domain.ValuationExpensive {
		add("near_high_expensive", "Near the 52-week high while valuation looks expensive.", -1, map[string]float64{FromHigh52W: round4(fh)})
	}
	switch view {
	case domain.ValuationCheap:
		add("valuation_cheap", "Valuation looks cheap.", 1, nil)
	case domain.ValuationExpensive:
		add("valuation_expensive", "Valuation looks expensive.", -1, nil)
	}

	total := 0.0
	for _, sig := range s {
		total += sig.Weight
	}
	view2 := domain.TimingWait
	switch {
	case total >= 1.5:
		view2 = domain.TimingAccumulate
	case total <= -1.5:
		view2 = domain.TimingAvoid
	}
	return domain.Timing{View: view2, Signals: s}
}
