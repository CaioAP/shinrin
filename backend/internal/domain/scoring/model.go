package scoring

import (
	"strings"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
)

// Direction says which way a metric is good for the investor.
type Direction int

const (
	HigherIsBetter Direction = iota
	LowerIsBetter
)

// MetricSpec is one metric's role in a factor.
type MetricSpec struct {
	Name      string
	Direction Direction
	Weight    float64
	// Absolute, when set, scores the metric on a fixed 0-100 scale instead
	// of ranking it against peers. Used where "good" has a meaning of its
	// own (a payout ratio near 100% is risky however common it is).
	Absolute func(v float64) float64
}

// Model is the metric set used to score one kind of asset. Each factor lists
// its metrics; a factor with no metric available is left unscored.
type Model struct {
	Name    string
	Factors map[domain.Factor][]MetricSpec
}

func hi(name string, w float64) MetricSpec {
	return MetricSpec{Name: name, Direction: HigherIsBetter, Weight: w}
}
func lo(name string, w float64) MetricSpec {
	return MetricSpec{Name: name, Direction: LowerIsBetter, Weight: w}
}

var (
	momentumSpecs = []MetricSpec{
		hi(indicators.Return3M, 0.5), hi(indicators.Return6M, 1), hi(indicators.Return12M, 1), hi(PriceVsSMA200, 1),
	}
	sentimentSpecs = []MetricSpec{{Name: NewsTone, Weight: 1, Absolute: func(v float64) float64 { return 50 + 50*v }}}
	payoutSpec     = MetricSpec{Name: PayoutRatio, Weight: 0.5, Absolute: payoutPoints}
)

// StockModel scores operating companies.
var StockModel = Model{Name: "stock", Factors: map[domain.Factor][]MetricSpec{
	domain.FactorValuation: {lo(indicators.PE, 1), lo(indicators.PB, 1), lo(indicators.EVToEBITDA, 1), hi(indicators.FCFYield, 1), hi(indicators.DividendYield, 0.5)},
	domain.FactorQuality:   {hi(indicators.ROE, 1), hi(indicators.NetMargin, 1), hi(indicators.EBITMargin, 0.5), lo(indicators.NetDebtToEBITDA, 1)},
	domain.FactorGrowth:    {hi(indicators.RevenueGrowth1Y, 1), hi(indicators.EarningsGrowth1Y, 1), hi(indicators.RevenueCAGR3Y, 1)},
	domain.FactorMomentum:  momentumSpecs,
	domain.FactorIncome:    {hi(indicators.DividendYield, 1), hi(DividendYears, 1), payoutSpec},
	domain.FactorRisk:      {lo(indicators.Volatility1Y, 1), hi(indicators.MaxDrawdown1Y, 1), lo(indicators.NetDebtToEBITDA, 0.5)},
	domain.FactorSentiment: sentimentSpecs,
}}

// FinancialsModel scores banks and insurers. EBITDA, EV and net debt do not
// describe a lender (debt is its raw material), so those metrics are dropped
// and the balance-sheet multiples carry valuation.
var FinancialsModel = Model{Name: "financials", Factors: map[domain.Factor][]MetricSpec{
	domain.FactorValuation: {lo(indicators.PE, 1), lo(indicators.PB, 1.5), hi(indicators.DividendYield, 0.5)},
	domain.FactorQuality:   {hi(indicators.ROE, 1.5), hi(indicators.NetMargin, 1)},
	domain.FactorGrowth:    {hi(indicators.RevenueGrowth1Y, 1), hi(indicators.EarningsGrowth1Y, 1), hi(indicators.RevenueCAGR3Y, 1)},
	domain.FactorMomentum:  momentumSpecs,
	domain.FactorIncome:    {hi(indicators.DividendYield, 1), hi(DividendYears, 1), payoutSpec},
	domain.FactorRisk:      {lo(indicators.Volatility1Y, 1), hi(indicators.MaxDrawdown1Y, 1)},
	domain.FactorSentiment: sentimentSpecs,
}}

// FundModel scores FIIs and REITs, which are bought for their distributions.
// Quality and growth need fund reports (vacancy, FFO) that are not ingested
// yet, so those factors stay unscored rather than guessed.
var FundModel = Model{Name: "fund", Factors: map[domain.Factor][]MetricSpec{
	domain.FactorValuation: {lo(indicators.PB, 1), hi(indicators.DividendYield, 1)},
	domain.FactorMomentum:  momentumSpecs,
	domain.FactorIncome:    {hi(indicators.DividendYield, 1), hi(DividendYears, 1)},
	domain.FactorRisk:      {lo(indicators.Volatility1Y, 1), hi(indicators.MaxDrawdown1Y, 1)},
	domain.FactorSentiment: sentimentSpecs,
}}

// ModelFor picks the metric set for an asset (a Strategy chosen by asset
// class and sector). ok is false for classes the engine does not score.
func ModelFor(a domain.Asset) (Model, bool) {
	switch a.Class {
	case domain.ClassStock:
		if IsFinancialSector(a.Sector) {
			return FinancialsModel, true
		}
		return StockModel, true
	case domain.ClassFII, domain.ClassREIT:
		return FundModel, true
	}
	return Model{}, false
}

// IsFinancialSector recognises banks, insurers and other lenders from the
// sector names the sources use (GICS "Financials", B3 "Financeiro",
// "Bancos", "Seguradoras").
func IsFinancialSector(sector string) bool {
	s := strings.ToLower(sector)
	for _, w := range []string{"financ", "bank", "banco", "insur", "segur"} {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// payoutPoints rewards a payout ratio that shares profits without starving
// the business: 30-70% is ideal, above 100% pays out more than it earns.
func payoutPoints(p float64) float64 {
	switch {
	case p <= 0:
		return 0
	case p < 0.3:
		return 50 + p/0.3*50
	case p <= 0.7:
		return 100
	case p <= 1:
		return 100 - (p-0.7)/0.3*60
	default:
		return max(0, 40-(p-1)*80)
	}
}
