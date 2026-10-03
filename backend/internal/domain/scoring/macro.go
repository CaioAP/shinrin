package scoring

import (
	"math"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Display units of a MacroIndicator.
const (
	UnitPctYear   = "pct_year"
	UnitBRLPerUSD = "brl_per_usd"
)

// MacroStripOf turns raw macro series (each sorted oldest first) into the
// headline numbers of the dashboard, in a fixed order. An indicator without
// enough data goes to Missing.
func MacroStripOf(series map[string][]domain.MacroPoint) domain.MacroStrip {
	var s domain.MacroStrip
	add := func(code string, v float64, ok bool, unit string, last domain.MacroPoint) {
		if !ok {
			s.Missing = append(s.Missing, code)
			return
		}
		s.Items = append(s.Items, domain.MacroIndicator{Code: code, Value: round2(v), Unit: unit, AsOf: last.Date, Source: last.Source})
	}
	latest := func(code string) (float64, bool, domain.MacroPoint) {
		pts := series[code]
		if len(pts) == 0 {
			return 0, false, domain.MacroPoint{}
		}
		return pts[len(pts)-1].Value, true, pts[len(pts)-1]
	}

	v, ok, last := latest(domain.SeriesSelicTarget)
	add("selic", v, ok, UnitPctYear, last)

	v, ok, last = latest(domain.SeriesCDI)
	add("cdi", AnnualizeDaily(v), ok, UnitPctYear, last)

	ipca := series[domain.SeriesIPCA]
	v, ok = Compound12M(ipca)
	if ok {
		last = ipca[len(ipca)-1]
	}
	add("ipca_12m", v, ok, UnitPctYear, last)

	v, ok, last = latest(domain.SeriesUSDBRL)
	add("usdbrl", v, ok, UnitBRLPerUSD, last)

	v, ok, last = latest(domain.SeriesFedFunds)
	add("fed_funds", v, ok, UnitPctYear, last)

	v, ok, last = latest(domain.SeriesUST10Y)
	add("ust_10y", v, ok, UnitPctYear, last)

	cpi := series[domain.SeriesUSCPI]
	v, ok = YearOverYear(cpi)
	if ok {
		last = cpi[len(cpi)-1]
	}
	add("us_cpi_12m", v, ok, UnitPctYear, last)
	return s
}

// AnnualizeDaily converts a daily rate in percent (how BCB publishes the
// CDI) to percent a year over 252 business days.
func AnnualizeDaily(pctDay float64) float64 {
	return (math.Pow(1+pctDay/100, 252) - 1) * 100
}

// YearOverYear is the percentage change of an index level (US CPI) between
// its newest point and the point a year earlier. points must be sorted
// oldest first; ok is false when no point sits within 45 days of a year
// before the newest.
func YearOverYear(points []domain.MacroPoint) (float64, bool) {
	if len(points) < 2 {
		return 0, false
	}
	last := points[len(points)-1]
	target := last.Date.AddDate(-1, 0, 0)
	for i := len(points) - 2; i >= 0; i-- {
		p := points[i]
		if p.Date.After(target) {
			continue
		}
		if target.Sub(p.Date) > 45*24*time.Hour || p.Value == 0 {
			return 0, false
		}
		return (last.Value/p.Value - 1) * 100, true
	}
	return 0, false
}
