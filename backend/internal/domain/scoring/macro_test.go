package scoring_test

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/scoring"
)

func monthly(series string, start time.Time, values ...float64) []domain.MacroPoint {
	out := make([]domain.MacroPoint, len(values))
	for i, v := range values {
		out[i] = domain.MacroPoint{Series: series, Date: start.AddDate(0, i, 0), Value: v, Source: "test"}
	}
	return out
}

func TestMacroStripOf(t *testing.T) {
	jan := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
	series := map[string][]domain.MacroPoint{
		domain.SeriesSelicTarget: monthly(domain.SeriesSelicTarget, jan, 14.75, 15),
		domain.SeriesCDI:         monthly(domain.SeriesCDI, jan, 0.055131),
		domain.SeriesIPCA:        monthly(domain.SeriesIPCA, jan, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5),
		domain.SeriesUSCPI:       monthly(domain.SeriesUSCPI, jan, 300, 301, 302, 303, 304, 305, 306, 307, 308, 309, 310, 311, 309),
	}
	s := scoring.MacroStripOf(series)

	got := map[string]float64{}
	for _, it := range s.Items {
		got[it.Code] = it.Value
	}
	want := map[string]float64{"selic": 15, "cdi": 14.9, "ipca_12m": 6.17, "us_cpi_12m": 3}
	for code, w := range want {
		if math.Abs(got[code]-w) > 0.01 {
			t.Errorf("%s = %v, want %v", code, got[code], w)
		}
	}
	if !slices.Equal(s.Missing, []string{"usdbrl", "fed_funds", "ust_10y"}) {
		t.Errorf("missing = %v", s.Missing)
	}
	if s.Items[0].Code != "selic" || !s.Items[0].AsOf.Equal(jan.AddDate(0, 1, 0)) {
		t.Errorf("first item = %+v", s.Items[0])
	}
}

func TestYearOverYearNeedsAYearOfData(t *testing.T) {
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, ok := scoring.YearOverYear(monthly(domain.SeriesUSCPI, jan, 300, 301, 302)); ok {
		t.Error("three months should not give a year-over-year change")
	}
}
