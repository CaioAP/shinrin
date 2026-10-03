package domain_test

import (
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func qe(y int, m time.Month) time.Time { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC) }

func TestFinancialsTTM(t *testing.T) {
	row := func(end time.Time, p domain.PeriodType, v float64) domain.Fundamental {
		return domain.Fundamental{Metric: domain.MetricRevenue, PeriodEnd: end, PeriodType: p, Value: v}
	}
	rows := []domain.Fundamental{
		row(qe(2024, 3), domain.PeriodQuarter, 10),
		row(qe(2024, 6), domain.PeriodQuarter, 20),
		row(qe(2024, 9), domain.PeriodQuarter, 30),
		row(qe(2024, 12), domain.PeriodFiscalYear, 100), // derived Q4 = 40
		row(qe(2025, 3), domain.PeriodQuarter, 15),
	}
	fin := domain.NewFinancials(rows)

	tests := []struct {
		name    string
		asOf    time.Time
		want    float64
		wantEnd time.Time
		ok      bool
	}{
		{name: "rolls Q1 2025 with derived Q4", asOf: qe(2025, 5), want: 20 + 30 + 40 + 15, wantEnd: qe(2025, 3), ok: true},
		{name: "fiscal year when it is the newest", asOf: qe(2025, 1), want: 100, wantEnd: qe(2024, 12), ok: true},
		{name: "not enough quarters and no year", asOf: qe(2024, 10)},
	}
	for _, tt := range tests {
		got, end, ok := fin.TTM(domain.MetricRevenue, tt.asOf)
		if ok != tt.ok || got != tt.want || !end.Equal(tt.wantEnd) {
			t.Errorf("%s: TTM = %v, %v, %v; want %v, %v, %v", tt.name, got, end, ok, tt.want, tt.wantEnd, tt.ok)
		}
	}
}

func TestFinancialsLatest(t *testing.T) {
	fin := domain.NewFinancials([]domain.Fundamental{
		{Metric: domain.MetricEquity, PeriodEnd: qe(2024, 12), Value: 5},
		{Metric: domain.MetricEquity, PeriodEnd: qe(2025, 3), Value: 6},
	})
	if v, _, ok := fin.Latest(domain.MetricEquity, qe(2025, 2)); !ok || v != 5 {
		t.Fatalf("Latest = %v, %v", v, ok)
	}
	if _, _, ok := fin.Latest(domain.MetricCash, qe(2025, 2)); ok {
		t.Fatal("unknown metric should not be ok")
	}
}

func TestAdjustCloses(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2025, 3, day, 0, 0, 0, 0, time.UTC) }
	bars := []domain.PriceBar{
		{Date: d(5), Close: 30}, // given out of order on purpose
		{Date: d(3), Close: 20},
		{Date: d(4), Close: 22},
	}
	actions := []domain.CorporateAction{
		{ExDate: d(4), Type: domain.ActionDividend, Value: 2}, // factor 1 - 2/20 = 0.9
		{ExDate: d(5), Type: domain.ActionSplit, Value: 2},    // factor 0.5
		{ExDate: d(1), Type: domain.ActionSplit, Value: 3},    // before the series: ignored
	}
	got := domain.AdjustCloses(bars, actions)
	want := []float64{20 * 0.9 * 0.5, 22 * 0.5, 30}
	for i, w := range want {
		if diff := got[i].AdjClose - w; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("bar %d (%s) AdjClose = %v, want %v", i, got[i].Date.Format("2006-01-02"), got[i].AdjClose, w)
		}
	}
	if bars[0].AdjClose != 0 {
		t.Error("input was modified")
	}
}
