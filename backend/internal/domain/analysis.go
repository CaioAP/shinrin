package domain

import (
	"fmt"
	"strings"
	"time"
)

// Factor is one independent angle an analyst scores an investment from
// (docs/design.md, section 8). Every factor score is 0 to 100, where higher is
// always better for the investor: a high valuation score means cheap, a high
// risk score means low risk.
type Factor string

const (
	FactorValuation Factor = "valuation"
	FactorQuality   Factor = "quality"
	FactorGrowth    Factor = "growth"
	FactorMomentum  Factor = "momentum"
	FactorIncome    Factor = "income"
	FactorRisk      Factor = "risk"
	FactorSentiment Factor = "sentiment"
)

// Factors lists every factor in display order.
var Factors = []Factor{FactorValuation, FactorQuality, FactorGrowth, FactorMomentum, FactorIncome, FactorRisk, FactorSentiment}

// ScoreInput is one metric that went into a factor score, kept so the UI can
// always answer "why this rating?".
type ScoreInput struct {
	Metric string
	Value  float64
	// Points is the metric's own 0-100 contribution: its percentile among
	// peers, or an absolute scale for metrics that are not peer-relative
	// (news tone).
	Points float64
	Weight float64
	// Peers is how many assets had this metric in the comparison group; zero
	// for absolute metrics.
	Peers int
}

// FactorScore is one factor's 0-100 score with its breakdown.
type FactorScore struct {
	Factor Factor
	Value  float64
	Inputs []ScoreInput
	// PeerGroup names who the asset was compared with, e.g. "B3 stock,
	// sector Energia" or "US stock (market-wide)".
	PeerGroup string
}

// Scorecard is every factor score computed for one asset on one date. A
// factor without enough data is absent, which is different from a low score.
type Scorecard struct {
	Asset   AssetKey
	AsOf    time.Time
	Factors []FactorScore
}

// Factor returns the score of f, if it was computed.
func (s Scorecard) Factor(f Factor) (FactorScore, bool) {
	for _, fs := range s.Factors {
		if fs.Factor == f {
			return fs, true
		}
	}
	return FactorScore{}, false
}

// RiskProfile is the investor profile from the suitability questionnaire.
type RiskProfile string

const (
	ProfileConservative RiskProfile = "conservative"
	ProfileModerate     RiskProfile = "moderate"
	ProfileAggressive   RiskProfile = "aggressive"
)

// ParseRiskProfile accepts a profile name in any case; empty means moderate,
// the default until a user has answered the questionnaire.
func ParseRiskProfile(s string) (RiskProfile, error) {
	switch p := RiskProfile(strings.ToLower(strings.TrimSpace(s))); p {
	case "":
		return ProfileModerate, nil
	case ProfileConservative, ProfileModerate, ProfileAggressive:
		return p, nil
	}
	return "", fmt.Errorf("%w: unknown risk profile %q", ErrInvalid, s)
}

// ValuationView summarises whether the price looks cheap or expensive.
type ValuationView string

const (
	ValuationCheap     ValuationView = "cheap"
	ValuationFair      ValuationView = "fair"
	ValuationExpensive ValuationView = "expensive"
)

// TimingView is the "when" signal: technicals and valuation combined.
type TimingView string

const (
	TimingAccumulate TimingView = "accumulate"
	TimingWait       TimingView = "wait"
	TimingAvoid      TimingView = "avoid"
)

// Signal is one rule that fired, with the data it read, so a view is never a
// bare verdict.
type Signal struct {
	Code    string // stable id, e.g. "uptrend", "rsi_overbought"
	Message string
	// Weight is the signal's contribution to the view: positive favours
	// buying now, negative favours waiting.
	Weight float64
	Data   map[string]float64
}

// Timing is a timing view with the signals behind it.
type Timing struct {
	View    TimingView
	Signals []Signal
}

// FairValue is one valuation model's estimate, always a range: a point
// target would suggest a precision these models do not have.
type FairValue struct {
	Method string // graham, bazin, dcf
	Low    float64
	High   float64
	// Assumptions are the inputs the estimate used (eps, discount rate, ...).
	Assumptions map[string]float64
}

// Analysis is everything the engine concludes about one asset for one risk
// profile. It is derived data: recomputed on read from the stored scorecard,
// indicators and macro series.
type Analysis struct {
	Asset   Asset
	AsOf    time.Time
	Profile RiskProfile
	Price   float64
	// Composite is the profile-weighted average of the factor scores, 0-100.
	Composite float64
	// Coverage is the share (0-1) of the profile's factor weights that had a
	// score; a composite built on little coverage deserves less trust.
	Coverage   float64
	Scorecard  Scorecard
	Indicators IndicatorSet
	FairValues []FairValue
	Valuation  ValuationView
	// ValuationSignals are the rules behind Valuation.
	ValuationSignals []Signal
	Timing           Timing
	// Notes explain what could not be computed and why (missing macro
	// series, no fundamentals for a fund, ...).
	Notes []string
}

// AllocationBand is a profile's suggested range for one asset class, in
// percent of the portfolio.
type AllocationBand struct {
	Class string // fixed_income, stocks, real_estate
	Min   float64
	Max   float64
	// Lean is where in the band macro conditions point: low, mid or high.
	Lean string
}

// Outlook is the allocation side of the engine: bands for a profile, tilted
// by macro conditions.
type Outlook struct {
	Profile RiskProfile
	AsOf    time.Time
	Bands   []AllocationBand
	Signals []Signal
	Notes   []string
}
