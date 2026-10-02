package domain_test

import (
	"errors"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func TestNewSymbol(t *testing.T) {
	tests := []struct {
		in      string
		want    domain.Symbol
		wantErr bool
	}{
		{in: "petr4", want: "PETR4"},
		{in: " MXRF11 ", want: "MXRF11"},
		{in: "BRK.B", want: "BRK.B"},
		{in: "", wantErr: true},
		{in: "drop table", wantErr: true},
	}
	for _, tt := range tests {
		got, err := domain.NewSymbol(tt.in)
		if tt.wantErr {
			if !errors.Is(err, domain.ErrInvalid) {
				t.Errorf("NewSymbol(%q) err = %v, want ErrInvalid", tt.in, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("NewSymbol(%q) = %q, %v, want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestParseMarket(t *testing.T) {
	if m, err := domain.ParseMarket("b3"); err != nil || m != domain.MarketB3 {
		t.Fatalf("ParseMarket(b3) = %q, %v", m, err)
	}
	if _, err := domain.ParseMarket("LSE"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("ParseMarket(LSE) err = %v, want ErrInvalid", err)
	}
	if got := domain.MarketB3.Currency(); got != "BRL" {
		t.Fatalf("B3 currency = %q", got)
	}
}

func TestIsB3Unit(t *testing.T) {
	for k, want := range map[domain.AssetKey]bool{
		{Market: domain.MarketB3, Symbol: "TAEE11"}: true,
		{Market: domain.MarketB3, Symbol: "PETR4"}:  false,
		{Market: domain.MarketUS, Symbol: "XX11"}:   false,
	} {
		if got := k.IsB3Unit(); got != want {
			t.Errorf("%s.IsB3Unit() = %v", k, got)
		}
	}
}
