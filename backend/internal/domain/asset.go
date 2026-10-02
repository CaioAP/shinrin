package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Market is the exchange or jurisdiction an asset trades in.
type Market string

const (
	MarketB3 Market = "B3"
	MarketUS Market = "US"
)

// ParseMarket accepts a market code in any case.
func ParseMarket(s string) (Market, error) {
	switch m := Market(strings.ToUpper(s)); m {
	case MarketB3, MarketUS:
		return m, nil
	}
	return "", fmt.Errorf("%w: unknown market %q", ErrInvalid, s)
}

// Currency returns the currency prices in this market are quoted in.
func (m Market) Currency() string {
	if m == MarketB3 {
		return "BRL"
	}
	return "USD"
}

// AssetClass groups assets that are analysed with the same metric set.
type AssetClass string

const (
	ClassStock   AssetClass = "stock"
	ClassFII     AssetClass = "fii"
	ClassREIT    AssetClass = "reit"
	ClassGovBond AssetClass = "gov_bond"
	ClassIndex   AssetClass = "index"
)

// ParseAssetClass accepts a class name in any case.
func ParseAssetClass(s string) (AssetClass, error) {
	switch c := AssetClass(strings.ToLower(s)); c {
	case ClassStock, ClassFII, ClassREIT, ClassGovBond, ClassIndex:
		return c, nil
	}
	return "", fmt.Errorf("%w: unknown asset class %q", ErrInvalid, s)
}

// Symbol is a ticker such as PETR4, MXRF11 or AAPL. It is always upper case.
type Symbol string

var symbolPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9.\-]{0,15}$`)

// NewSymbol normalises and validates a ticker.
func NewSymbol(s string) (Symbol, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !symbolPattern.MatchString(s) {
		return "", fmt.Errorf("%w: bad symbol %q", ErrInvalid, s)
	}
	return Symbol(s), nil
}

// AssetKey identifies an asset across markets (the same ticker can exist in
// more than one market).
type AssetKey struct {
	Market Market
	Symbol Symbol
}

func (k AssetKey) String() string { return string(k.Market) + ":" + string(k.Symbol) }

// Asset is anything Shinrin tracks and scores.
type Asset struct {
	Key         AssetKey
	Class       AssetClass
	Name        string
	Sector      string
	ISIN        string
	IndexMember bool
	Active      bool
}

// Currency is the asset's quote currency.
func (a Asset) Currency() string { return a.Key.Market.Currency() }
