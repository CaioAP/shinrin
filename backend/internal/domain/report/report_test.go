package report_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/report"
	"github.com/CaioAP/shinrin/backend/internal/domain/scoring"
)

func analysis() (domain.Analysis, domain.Outlook) {
	a := domain.Analysis{
		Asset:     domain.Asset{Key: domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}, Class: domain.ClassStock, Name: "Petrobras", Sector: "Energy"},
		AsOf:      time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		Profile:   domain.ProfileModerate,
		Price:     38.5,
		Composite: 71.3,
		Scorecard: domain.Scorecard{Factors: []domain.FactorScore{{Factor: domain.FactorValuation, Value: 88, PeerGroup: "B3 stock, sector Energy"}}},
		Indicators: domain.IndicatorSet{Values: map[string]float64{
			"pe": 4.123456, "roe": 0.2234, "market_cap": 4.53e11, "dividend_yield": 0.141,
		}},
		FairValues: []domain.FairValue{{Method: "bazin", Low: 67.86, High: 90.48, Assumptions: map[string]float64{"min_yield": 0.06}}},
		Valuation:  domain.ValuationCheap,
		Timing: domain.Timing{View: domain.TimingWait, Signals: []domain.Signal{
			{Code: "rsi_overbought", Message: "RSI is above 70: the recent rise looks stretched.", Data: map[string]float64{"rsi_14": 74.2}},
		}},
	}
	o := scoring.OutlookFor(domain.ProfileModerate, scoring.MacroContext{})
	return a, o
}

func TestSnapshot(t *testing.T) {
	a, o := analysis()
	news := []domain.NewsItem{{Title: "Petrobras raises dividend"}, {Title: "Second"}, {Title: "Third"}}
	s := report.NewSnapshot(a, o, news, 2)
	for k, want := range map[string]float64{"pe": 4.1235, "score_valuation": 88, "fair_bazin_low": 67.86, "band_max_pct": 50, "rsi_14": 74.2, "price": 38.5} {
		if got, ok := s.Facts[k]; !ok || got != want {
			t.Errorf("fact %s = %v, %v; want %v", k, got, ok, want)
		}
	}
	if s.Labels["valuation_view"] != "cheap" || s.Labels["band_class"] != "stocks" || !strings.Contains(s.Labels["signal_rsi_overbought"], "70") {
		t.Errorf("labels = %v", s.Labels)
	}
	if len(s.Headlines) != 2 || s.Headlines[1].ID != "news:2" || !s.Has("news:2") || s.Has("news:3") {
		t.Errorf("headlines = %+v", s.Headlines)
	}
}

func goodOutput() domain.ReportOutput {
	return domain.ReportOutput{
		Summary:          "PETR4 trades at a P/E of 4.1x with an ROE of 22.3%, and a dividend yield near 14.1%; the market cap is about R$ 450 billion.",
		BullCase:         []string{"Price of R$ 38.50 sits below the Bazin range of 67.86 to 90.48.", "Valuation score of 88 against Energy peers."},
		BearCase:         []string{"RSI at 74.2 is above 70, so the 200-day trend may pause in Q4."},
		ValuationView:    "cheap",
		TimingView:       "wait",
		FitForProfile:    "good",
		AllocationMinPct: 2,
		AllocationMaxPct: 5,
		KeyRisks:         []string{"Political interference in pricing (2 risks noted in 2026)."},
		Confidence:       "medium",
		CitedData:        []string{"pe", "roe", "fair_bazin_low", "news:1"},
	}
}

func TestValidateAcceptsGroundedReport(t *testing.T) {
	a, o := analysis()
	s := report.NewSnapshot(a, o, []domain.NewsItem{{Title: "x"}}, 5)
	if issues := report.Validate(goodOutput(), s); len(issues) != 0 {
		t.Fatalf("issues = %v", issues)
	}
}

func TestValidateRejectsInventedNumbers(t *testing.T) {
	a, o := analysis()
	s := report.NewSnapshot(a, o, nil, 5)
	out := goodOutput()
	out.Summary = "Analysts expect 35% upside and EPS of R$ 9.40."
	out.BullCase = append(out.BullCase, "Debt fell to 1,234.5 million.")
	out.ValuationView = "bargain"
	out.AllocationMaxPct = 60 // band max is 50
	out.CitedData = []string{"pe", "eps_forecast"}

	issues := report.Validate(out, s)
	var got []string
	for _, is := range issues {
		got = append(got, is.String())
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"summary: numbers not in the data: 35%, 9.40", "bull_case[2]: numbers not in the data: 1,234.5 million", "valuation_view:", "suggested_allocation_pct:", `cited_data[1]: "eps_forecast"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing issue %q in:\n%s", want, joined)
		}
	}

	cleaned, omitted := report.Omit(out, issues)
	if cleaned.Summary != "" || len(cleaned.BullCase) != 2 || cleaned.ValuationView != "" || cleaned.AllocationMaxPct != 0 || !slices.Equal(cleaned.CitedData, []string{"pe"}) {
		t.Errorf("cleaned = %+v", cleaned)
	}
	if !slices.Contains(omitted, "bull_case[2]") || !slices.Contains(omitted, "summary") {
		t.Errorf("omitted = %v", omitted)
	}
}

func TestNumberFormats(t *testing.T) {
	a, o := analysis()
	s := report.NewSnapshot(a, o, []domain.NewsItem{{Title: "x"}}, 5)
	out := goodOutput()
	out.Summary = "ROE de 22,3% e P/L de 4,12; valor de mercado de R$ 453 bilhões. Média de 50 dias e máxima de 52 semanas."
	if issues := report.Validate(out, s); len(issues) != 0 {
		t.Errorf("Portuguese formats: %v", issues)
	}
}
