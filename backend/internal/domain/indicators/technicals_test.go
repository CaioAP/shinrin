package indicators_test

import (
	"math"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/domain/indicators"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSMA(t *testing.T) {
	tests := []struct {
		values []float64
		n      int
		want   float64
		ok     bool
	}{
		{values: []float64{1, 2, 3, 4}, n: 2, want: 3.5, ok: true},
		{values: []float64{1, 2, 3, 4}, n: 4, want: 2.5, ok: true},
		{values: []float64{1, 2}, n: 3},
		{values: []float64{1, 2}, n: 0},
	}
	for _, tt := range tests {
		got, ok := indicators.SMA(tt.values, tt.n)
		if ok != tt.ok || !near(got, tt.want) {
			t.Errorf("SMA(%v, %d) = %v, %v; want %v, %v", tt.values, tt.n, got, ok, tt.want, tt.ok)
		}
	}
}

func TestEMASeries(t *testing.T) {
	got := indicators.EMASeries([]float64{2, 4, 6, 8}, 3)
	// seed = SMA(2,4,6) = 4; k = 0.5; next = 8*0.5 + 4*0.5 = 6
	if !math.IsNaN(got[0]) || !math.IsNaN(got[1]) || !near(got[2], 4) || !near(got[3], 6) {
		t.Fatalf("EMASeries = %v", got)
	}
}

func TestRSI(t *testing.T) {
	up := make([]float64, 30)
	for i := range up {
		up[i] = float64(i + 1)
	}
	if v, ok := indicators.RSI(up, 14); !ok || v != 100 {
		t.Errorf("RSI(rising) = %v, %v; want 100", v, ok)
	}
	alt := make([]float64, 31)
	for i := range alt {
		alt[i] = 10 + float64(i%2) // equal gains and losses
	}
	if v, ok := indicators.RSI(alt, 14); !ok || math.Abs(v-50) > 2 {
		t.Errorf("RSI(alternating) = %v, %v; want about 50", v, ok)
	}
	if _, ok := indicators.RSI(up[:14], 14); ok {
		t.Error("RSI with n values should need n+1")
	}
}

func TestMACD(t *testing.T) {
	flat := make([]float64, 60)
	for i := range flat {
		flat[i] = 42
	}
	m, ok := indicators.MACD(flat, 12, 26, 9)
	if !ok || !near(m.MACD, 0) || !near(m.Signal, 0) || !near(m.Histogram, 0) {
		t.Fatalf("MACD(flat) = %+v, %v", m, ok)
	}
	rising := make([]float64, 60)
	for i := range rising {
		rising[i] = float64(i)
	}
	if m, ok := indicators.MACD(rising, 12, 26, 9); !ok || m.MACD <= 0 {
		t.Fatalf("MACD(rising) = %+v, %v; want positive line", m, ok)
	}
	if _, ok := indicators.MACD(rising[:20], 12, 26, 9); ok {
		t.Fatal("MACD should need slow+signal-1 values")
	}
}

func TestReturnVolatilityDrawdown(t *testing.T) {
	if r, ok := indicators.Return([]float64{100, 105, 110}, 2); !ok || !near(r, 0.1) {
		t.Errorf("Return = %v, %v", r, ok)
	}
	if v, ok := indicators.Volatility([]float64{100, 100, 100, 100}, 3); !ok || v != 0 {
		t.Errorf("Volatility(flat) = %v, %v", v, ok)
	}
	if v, ok := indicators.Volatility([]float64{100, 110, 100, 110, 100}, 4); !ok || v <= 0 {
		t.Errorf("Volatility(choppy) = %v, %v", v, ok)
	}
	if d, ok := indicators.MaxDrawdown([]float64{100, 120, 60, 90, 130}); !ok || !near(d, -0.5) {
		t.Errorf("MaxDrawdown = %v, %v; want -0.5", d, ok)
	}
	if h, l, ok := indicators.HighLow([]float64{5, 9, 1, 7}, 3); !ok || h != 9 || l != 1 {
		t.Errorf("HighLow = %v %v %v", h, l, ok)
	}
}
