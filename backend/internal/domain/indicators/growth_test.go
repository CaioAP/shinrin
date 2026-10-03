package indicators_test

import (
	"math"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
)

func fy(year int, metric string, v float64) domain.Fundamental {
	return domain.Fundamental{Metric: metric, PeriodEnd: time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC), PeriodType: domain.PeriodFiscalYear, Value: v}
}

func TestGrowth(t *testing.T) {
	fin := domain.NewFinancials([]domain.Fundamental{
		fy(2021, domain.MetricRevenue, 100), fy(2023, domain.MetricRevenue, 110), fy(2024, domain.MetricRevenue, 133.1),
		fy(2023, domain.MetricNetIncome, -5), fy(2024, domain.MetricNetIncome, 10),
	})
	got := indicators.Growth(fin, time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
	if v := got[indicators.RevenueGrowth1Y]; math.Abs(v-0.21) > 1e-9 {
		t.Errorf("revenue growth = %v, want 0.21", v)
	}
	if v := got[indicators.RevenueCAGR3Y]; math.Abs(v-(math.Pow(1.331, 1.0/3)-1)) > 1e-9 {
		t.Errorf("revenue CAGR = %v", v)
	}
	if _, ok := got[indicators.EarningsGrowth1Y]; ok {
		t.Error("earnings growth from a loss should be absent")
	}

	// A gap year (2022 missing) must not be bridged with an older period.
	gap := indicators.Growth(domain.NewFinancials([]domain.Fundamental{fy(2021, domain.MetricRevenue, 100), fy(2023, domain.MetricRevenue, 120)}), time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	if _, ok := gap[indicators.RevenueGrowth1Y]; ok {
		t.Errorf("growth across a gap = %v", gap)
	}
}
