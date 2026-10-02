// Package indicators computes the standard numbers analysts screen with:
// technicals from a price series (moving averages, RSI, MACD, returns,
// volatility, drawdown) and valuation and quality ratios from fundamentals
// (P/E, P/B, EV/EBITDA, dividend yield, ROE, margins, leverage).
//
// Everything here is a pure function of its inputs, with no I/O, so it is
// reproducible and the scoring engine can be tested against fixed numbers.
// Functions return ok=false rather than a misleading number when there is not
// enough data or a ratio is meaningless (P/E with a loss, for example).
package indicators
