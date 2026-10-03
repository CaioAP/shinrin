package scoring_test

import (
	"math"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
	"github.com/CaioAP/shinrin/backend/internal/domain/scoring"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestPercentile(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5}
	tests := []struct {
		v, want float64
	}{{1, 0}, {3, 50}, {5, 100}, {2.5, 40}}
	for _, tt := range tests {
		if got := scoring.Percentile(vals, tt.v); !near(got, tt.want, 1e-9) {
			t.Errorf("Percentile(%v) = %v, want %v", tt.v, got, tt.want)
		}
	}
	if got := scoring.Percentile([]float64{7, 7, 7}, 7); got != 50 {
		t.Errorf("ties = %v, want 50", got)
	}
}

func stock(sym, sector string) domain.Asset {
	return domain.Asset{Key: domain.AssetKey{Market: domain.MarketB3, Symbol: domain.Symbol(sym)}, Class: domain.ClassStock, Sector: sector, Active: true}
}

func TestScoreRanksWithinPeersAndRespectsDirection(t *testing.T) {
	asOf := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	var inputs []scoring.Input
	// Five energy stocks: P/E 4..12, so the cheapest (E1) gets the best
	// valuation points. Two retail stocks are too few for a sector group.
	for i, sym := range []string{"E1", "E2", "E3", "E4", "E5"} {
		inputs = append(inputs, scoring.Input{Asset: stock(sym, "Energy"), AsOf: asOf, Metrics: map[string]float64{
			indicators.PE: float64(4 + 2*i), indicators.ROE: 0.1 + 0.02*float64(i),
		}})
	}
	inputs = append(inputs,
		scoring.Input{Asset: stock("R1", "Retail"), AsOf: asOf, Metrics: map[string]float64{indicators.PE: 30}},
		scoring.Input{Asset: stock("R2", "Retail"), AsOf: asOf, Metrics: map[string]float64{indicators.PE: 3}},
		scoring.Input{Asset: domain.Asset{Key: domain.AssetKey{Market: domain.MarketB3, Symbol: "IBOV"}, Class: domain.ClassIndex}, AsOf: asOf},
	)
	cards := scoring.Score(inputs)
	if len(cards) != 7 {
		t.Fatalf("got %d scorecards, want 7 (index skipped)", len(cards))
	}
	by := map[domain.Symbol]domain.Scorecard{}
	for _, c := range cards {
		by[c.Asset.Symbol] = c
	}
	v1, _ := by["E1"].Factor(domain.FactorValuation)
	v5, _ := by["E5"].Factor(domain.FactorValuation)
	if v1.Value != 100 || v5.Value != 0 {
		t.Errorf("valuation E1 = %v, E5 = %v; want 100 and 0 (lower P/E is better)", v1.Value, v5.Value)
	}
	if v1.PeerGroup != "B3 stock, sector Energy" || v1.Inputs[0].Peers != 5 {
		t.Errorf("E1 peers = %q, %+v", v1.PeerGroup, v1.Inputs)
	}
	q5, _ := by["E5"].Factor(domain.FactorQuality)
	if q5.Value != 100 {
		t.Errorf("quality E5 = %v, want 100 (higher ROE is better)", q5.Value)
	}
	r2, ok := by["R2"].Factor(domain.FactorValuation)
	if !ok || r2.PeerGroup != "B3 stock (market-wide)" || r2.Value != 100 || r2.Inputs[0].Peers != 7 {
		t.Errorf("R2 valuation = %+v, %v; want market-wide rank, best P/E of 7", r2, ok)
	}
	if _, ok := by["R1"].Factor(domain.FactorGrowth); ok {
		t.Error("growth without metrics should be absent, not zero")
	}
}

func TestModelFor(t *testing.T) {
	if m, _ := scoring.ModelFor(stock("ITUB4", "Financeiro")); m.Name != "financials" {
		t.Errorf("bank model = %s", m.Name)
	}
	if m, _ := scoring.ModelFor(domain.Asset{Class: domain.ClassFII}); m.Name != "fund" {
		t.Errorf("FII model = %s", m.Name)
	}
	if _, ok := scoring.ModelFor(domain.Asset{Class: domain.ClassGovBond}); ok {
		t.Error("bonds are not scored by factor models")
	}
}

func TestComposite(t *testing.T) {
	sc := domain.Scorecard{Factors: []domain.FactorScore{
		{Factor: domain.FactorValuation, Value: 80}, {Factor: domain.FactorGrowth, Value: 20},
	}}
	// Conservative: valuation 0.20, growth 0.05 -> (16 + 1) / 0.25 = 68.
	c, cov := scoring.Composite(sc, domain.ProfileConservative)
	if c != 68 || cov != 0.25 {
		t.Errorf("conservative = %v, %v", c, cov)
	}
	// Aggressive: valuation 0.15, growth 0.25 -> (12 + 5) / 0.40 = 42.5.
	if c, _ := scoring.Composite(sc, domain.ProfileAggressive); c != 42.5 {
		t.Errorf("aggressive = %v", c)
	}
	for p, w := range scoring.ProfileWeights {
		sum := 0.0
		for _, v := range w {
			sum += v
		}
		if !near(sum, 1, 1e-9) {
			t.Errorf("%s weights sum to %v", p, sum)
		}
	}
}

func TestMetricsAndDividendYears(t *testing.T) {
	m := scoring.Metrics(map[string]float64{
		indicators.Close: 110, indicators.SMA200: 100, indicators.High52W: 120, indicators.Low52W: 100,
		indicators.DividendYield: 0.05, indicators.PE: 10,
	}, map[string]float64{scoring.NewsTone: 0.2})
	if !near(m[scoring.PriceVsSMA200], 0.1, 1e-9) || !near(m[scoring.PayoutRatio], 0.5, 1e-9) || m[scoring.NewsTone] != 0.2 {
		t.Errorf("Metrics = %v", m)
	}
	if _, ok := m[scoring.PriceVsSMA50]; ok {
		t.Error("missing SMA50 should not produce a ratio")
	}
	d := func(y int) time.Time { return time.Date(y, 6, 1, 0, 0, 0, 0, time.UTC) }
	acts := []domain.CorporateAction{
		{ExDate: d(2022), Type: domain.ActionDividend, Value: 1}, {ExDate: d(2022).AddDate(0, 3, 0), Type: domain.ActionJCP, Value: 1},
		{ExDate: d(2024), Type: domain.ActionJCP, Value: 1}, {ExDate: d(2025), Type: domain.ActionSplit, Value: 2},
		{ExDate: d(2019), Type: domain.ActionDividend, Value: 1},
	}
	if got := scoring.CountDividendYears(acts, d(2026)); got != 2 {
		t.Errorf("CountDividendYears = %v, want 2", got)
	}
}

func TestTone(t *testing.T) {
	if v, ok := scoring.Tone("Petrobras reports record profit, dividends beat estimates"); !ok || v != 1 {
		t.Errorf("positive = %v %v", v, ok)
	}
	if v, ok := scoring.Tone("Vale tem prejuízo e ações em queda"); !ok || v != -1 {
		t.Errorf("negative pt = %v %v", v, ok)
	}
	if _, ok := scoring.Tone("Company to hold annual meeting"); ok {
		t.Error("neutral text should carry no signal")
	}
	tone, n, ok := scoring.NewsToneOf([]domain.NewsItem{{Title: "profit surges"}, {Title: "lawsuit filed"}, {Title: "meeting"}})
	if !ok || tone != 0 || n != 3 {
		t.Errorf("NewsToneOf = %v %v %v", tone, n, ok)
	}
}

func TestFairValues(t *testing.T) {
	ind := map[string]float64{
		indicators.Close: 40, indicators.PE: 8, indicators.PB: 1, // EPS 5, BVPS 40
		indicators.DividendYield: 0.1, // DPS 4
		indicators.FCFYield:      0.1, // FCF/share 4
		indicators.RevenueCAGR3Y: 0.05,
	}
	fvs, notes := scoring.FairValues(domain.MarketB3, ind, 10, true)
	if len(fvs) != 3 || len(notes) != 0 {
		t.Fatalf("fair values = %+v, notes %v", fvs, notes)
	}
	graham := math.Sqrt(22.5 * 5 * 40) // 67.08
	if !near(fvs[0].High, graham, 0.01) || !near(fvs[0].Low, graham*2/3, 0.01) {
		t.Errorf("graham = %+v", fvs[0])
	}
	if fvs[1].Low != 50 || !near(fvs[1].High, 66.67, 0.01) {
		t.Errorf("bazin = %+v", fvs[1])
	}
	if d := fvs[2]; d.Method != "dcf" || d.Low >= d.High || d.Low <= 0 || d.Assumptions["discount_rate"] != 0.16 {
		t.Errorf("dcf = %+v", d)
	}

	// Without a risk-free rate the DCF is skipped with a note.
	fvs, notes = scoring.FairValues(domain.MarketB3, ind, 0, false)
	if len(fvs) != 2 || len(notes) != 1 {
		t.Errorf("no risk-free: %+v %v", fvs, notes)
	}
}

func TestValuationAndTiming(t *testing.T) {
	sc := domain.Scorecard{Factors: []domain.FactorScore{{Factor: domain.FactorValuation, Value: 80}}}
	fvs := []domain.FairValue{{Low: 50, High: 70}, {Low: 45, High: 60}}
	view, sig := scoring.ValuationViewOf(sc, 40, fvs)
	if view != domain.ValuationCheap || len(sig) != 2 {
		t.Errorf("view = %v %+v", view, sig)
	}
	if view, _ := scoring.ValuationViewOf(domain.Scorecard{}, 65, fvs); view != domain.ValuationFair {
		t.Errorf("one model above, one inside = %v, want fair", view)
	}

	up := map[string]float64{
		scoring.PriceVsSMA200: 0.1, indicators.SMA50: 105, indicators.SMA200: 100, indicators.MACDHistogram: 0.2, indicators.RSI14: 55,
	}
	if tm := scoring.TimingOf(up, domain.ValuationFair); tm.View != domain.TimingAccumulate || len(tm.Signals) != 2 {
		t.Errorf("uptrend fair = %+v", tm)
	}
	up[indicators.RSI14] = 78
	if tm := scoring.TimingOf(up, domain.ValuationFair); tm.View != domain.TimingWait {
		t.Errorf("overbought = %+v", tm)
	}
	down := map[string]float64{
		scoring.PriceVsSMA200: -0.2, indicators.SMA50: 90, indicators.SMA200: 100, indicators.MACDHistogram: -0.1,
		scoring.FromHigh52W: -0.25, scoring.FromLow52W: 0.02,
	}
	if tm := scoring.TimingOf(down, domain.ValuationFair); tm.View != domain.TimingAvoid {
		t.Errorf("downtrend = %+v", tm)
	}
	if tm := scoring.TimingOf(down, domain.ValuationCheap); tm.View != domain.TimingWait || tm.Signals[len(tm.Signals)-2].Code != "near_low_cheap" {
		t.Errorf("downtrend but cheap near the low = %+v", tm)
	}
	if tm := scoring.TimingOf(map[string]float64{}, domain.ValuationFair); tm.View != domain.TimingWait || len(tm.Signals) != 0 {
		t.Errorf("no data = %+v", tm)
	}
}

func TestOutlook(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	o := scoring.OutlookFor(domain.ProfileModerate, scoring.MacroContext{Selic: f(15), IPCA12M: f(5), UST10Y: f(4), UST3M: f(4.5)})
	lean := map[string]string{}
	for _, b := range o.Bands {
		lean[b.Class] = b.Lean
	}
	if lean[scoring.ClassFixedIncome] != "high" || lean[scoring.ClassStocks] != "low" || lean[scoring.ClassRealEstate] != "low" {
		t.Errorf("high real rate + inverted curve leans = %v", lean)
	}
	if len(o.Signals) != 2 || o.Signals[0].Code != "high_real_rate_br" || o.Signals[1].Code != "us_curve_inverted" {
		t.Errorf("signals = %+v", o.Signals)
	}

	// Missing Brazilian series: bands stay mid, with a note.
	o = scoring.OutlookFor(domain.ProfileConservative, scoring.MacroContext{})
	if len(o.Notes) != 2 || o.Bands[0].Lean != "mid" || o.Bands[0].Min != 60 {
		t.Errorf("no macro = %+v", o)
	}
	if scoring.Bands[domain.ProfileConservative][0].Lean != "" {
		t.Error("OutlookFor must not modify the shared bands")
	}
}

func TestCompound12M(t *testing.T) {
	var pts []domain.MacroPoint
	for range 13 {
		pts = append(pts, domain.MacroPoint{Value: 0.5})
	}
	if v, ok := scoring.Compound12M(pts); !ok || !near(v, (math.Pow(1.005, 12)-1)*100, 1e-9) {
		t.Errorf("Compound12M = %v %v", v, ok)
	}
	if _, ok := scoring.Compound12M(pts[:11]); ok {
		t.Error("needs twelve months")
	}
}
