package indicators

import "math"

// TradingDaysPerYear is used to annualise volatility and size return windows.
const TradingDaysPerYear = 252

// SMA returns the simple moving average of the last n values.
func SMA(values []float64, n int) (float64, bool) {
	if n <= 0 || len(values) < n {
		return 0, false
	}
	sum := 0.0
	for _, v := range values[len(values)-n:] {
		sum += v
	}
	return sum / float64(n), true
}

// EMASeries returns the exponential moving average of values with period n.
// The first n-1 entries are NaN; entry n-1 is seeded with the SMA of the first
// n values, the usual convention in charting packages.
func EMASeries(values []float64, n int) []float64 {
	out := make([]float64, len(values))
	if n <= 0 || len(values) < n {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	k := 2 / float64(n+1)
	sum := 0.0
	for i := range n {
		sum += values[i]
		out[i] = math.NaN()
	}
	out[n-1] = sum / float64(n)
	for i := n; i < len(values); i++ {
		out[i] = values[i]*k + out[i-1]*(1-k)
	}
	return out
}

// RSI returns Wilder's relative strength index over n periods (14 is
// standard) for the last value. Needs at least n+1 values.
func RSI(values []float64, n int) (float64, bool) {
	if n <= 0 || len(values) < n+1 {
		return 0, false
	}
	var gain, loss float64
	for i := 1; i <= n; i++ {
		d := values[i] - values[i-1]
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	gain /= float64(n)
	loss /= float64(n)
	for i := n + 1; i < len(values); i++ {
		d := values[i] - values[i-1]
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		gain = (gain*float64(n-1) + g) / float64(n)
		loss = (loss*float64(n-1) + l) / float64(n)
	}
	if loss == 0 {
		return 100, true
	}
	rs := gain / loss
	return 100 - 100/(1+rs), true
}

// MACDResult is the last point of a MACD calculation.
type MACDResult struct {
	MACD, Signal, Histogram float64
}

// MACD computes the moving average convergence divergence (12, 26, 9 is
// standard) for the last value.
func MACD(values []float64, fast, slow, signal int) (MACDResult, bool) {
	if fast <= 0 || slow <= fast || signal <= 0 || len(values) < slow+signal-1 {
		return MACDResult{}, false
	}
	ef, es := EMASeries(values, fast), EMASeries(values, slow)
	line := make([]float64, 0, len(values)-slow+1)
	for i := slow - 1; i < len(values); i++ {
		line = append(line, ef[i]-es[i])
	}
	sig := EMASeries(line, signal)
	m, s := line[len(line)-1], sig[len(sig)-1]
	return MACDResult{MACD: m, Signal: s, Histogram: m - s}, true
}

// Return is the simple return over the last n periods: last / value n periods
// earlier - 1.
func Return(values []float64, n int) (float64, bool) {
	if n <= 0 || len(values) < n+1 {
		return 0, false
	}
	base := values[len(values)-1-n]
	if base <= 0 {
		return 0, false
	}
	return values[len(values)-1]/base - 1, true
}

// Volatility is the annualised standard deviation of daily log returns over
// the last n returns (n+1 prices).
func Volatility(values []float64, n int) (float64, bool) {
	if n < 2 || len(values) < n+1 {
		return 0, false
	}
	w := values[len(values)-n-1:]
	rets := make([]float64, 0, n)
	for i := 1; i < len(w); i++ {
		if w[i-1] <= 0 || w[i] <= 0 {
			return 0, false
		}
		rets = append(rets, math.Log(w[i]/w[i-1]))
	}
	mean := 0.0
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))
	ss := 0.0
	for _, r := range rets {
		ss += (r - mean) * (r - mean)
	}
	return math.Sqrt(ss/float64(len(rets)-1)) * math.Sqrt(TradingDaysPerYear), true
}

// MaxDrawdown is the largest peak-to-trough fall over values, as a negative
// fraction (-0.35 means the price fell 35% from a previous high). Zero means
// it never fell.
func MaxDrawdown(values []float64) (float64, bool) {
	if len(values) < 2 {
		return 0, false
	}
	peak, worst := values[0], 0.0
	for _, v := range values {
		peak = max(peak, v)
		if peak > 0 {
			worst = min(worst, v/peak-1)
		}
	}
	return worst, true
}

// HighLow returns the highest and lowest of the last n values.
func HighLow(values []float64, n int) (high, low float64, ok bool) {
	if n <= 0 || len(values) < n {
		return 0, 0, false
	}
	w := values[len(values)-n:]
	high, low = w[0], w[0]
	for _, v := range w {
		high, low = max(high, v), min(low, v)
	}
	return high, low, true
}
