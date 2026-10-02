package domain

import (
	"slices"
	"sort"
)

// AdjustCloses returns a copy of bars, sorted by date, with AdjClose set to
// Close back-adjusted for corporate actions, so returns and long moving
// averages are not distorted by dividends or splits. This is the usual
// multiplicative method: every bar before an ex-date is scaled by
//
//	dividend or JCP:  1 - value / previous close
//	split or bonus:   1 / ratio   (ratio = shares after / shares before)
//
// Sources that already publish adjusted prices (Tiingo) do not need this; raw
// exchange files (B3 COTAHIST) do.
func AdjustCloses(bars []PriceBar, actions []CorporateAction) []PriceBar {
	out := slices.Clone(bars)
	slices.SortFunc(out, func(a, b PriceBar) int { return a.Date.Compare(b.Date) })
	n := len(out)
	if n == 0 {
		return out
	}

	// factor[i] scales every bar before i (the ex-date bar itself keeps it).
	factor := make([]float64, n)
	for i := range factor {
		factor[i] = 1
	}
	for _, a := range actions {
		i := sort.Search(n, func(j int) bool { return !out[j].Date.Before(a.ExDate) })
		if i == 0 || i == n {
			continue // no bar before the ex-date, or the ex-date is in the future
		}
		var f float64
		switch a.Type {
		case ActionDividend, ActionJCP:
			prev := out[i-1].Close
			if prev <= 0 {
				continue
			}
			f = 1 - a.Value/prev
		case ActionSplit, ActionBonus:
			if a.Value <= 0 {
				continue
			}
			f = 1 / a.Value
		}
		if f > 0 && f != 1 {
			factor[i] *= f
		}
	}

	cum := 1.0
	for i := n - 1; i >= 0; i-- {
		out[i].AdjClose = out[i].Close * cum
		cum *= factor[i]
	}
	return out
}
