package domain

import "time"

// Report kinds (docs/design.md, section 9). Only asset deep dives exist so
// far; watchlist reviews and news digests need accounts.
const ReportKindAsset = "asset"

// Headline is a news item as shown to the LLM, with the id it must cite.
type Headline struct {
	ID          string // "news:1"
	Title       string
	Source      string
	PublishedAt time.Time
}

// ReportSnapshot is the complete input of an AI report. The model may only
// use these numbers and labels, and the snapshot is stored with the report
// so every claim can be traced back to data.
type ReportSnapshot struct {
	// Facts are numbers keyed by a stable name (pe, score_quality,
	// fair_bazin_low, ...). Ratios are fractions.
	Facts map[string]float64
	// Labels are non-numeric data (name, sector, valuation_view, signal
	// messages).
	Labels    map[string]string
	Headlines []Headline
}

// Has reports whether key names a fact, a label or a headline.
func (s ReportSnapshot) Has(key string) bool {
	if _, ok := s.Facts[key]; ok {
		return true
	}
	if _, ok := s.Labels[key]; ok {
		return true
	}
	for _, h := range s.Headlines {
		if h.ID == key {
			return true
		}
	}
	return false
}

// ReportOutput is the analyst-style report the LLM writes, in the structure
// the design fixes.
type ReportOutput struct {
	Summary       string
	BullCase      []string
	BearCase      []string
	ValuationView string // cheap | fair | expensive
	TimingView    string // accumulate | wait | avoid
	FitForProfile string // good | partial | poor
	// AllocationMinPct and AllocationMaxPct bound the suggested position as a
	// share of the portfolio.
	AllocationMinPct float64
	AllocationMaxPct float64
	KeyRisks         []string
	Confidence       string // low | medium | high
	CitedData        []string
}

// Report is a stored AI report.
type Report struct {
	ID int64
	// UserID owns the report; zero for reports run from the CLI.
	UserID   UserID
	Kind     string
	Asset    AssetKey
	Profile  RiskProfile
	AsOf     time.Time // the data's as-of date, not the generation time
	Snapshot ReportSnapshot
	Output   ReportOutput
	// Omitted names sections removed because they failed validation twice.
	Omitted   []string
	Provider  string
	Model     string
	TokensIn  int
	TokensOut int
	CreatedAt time.Time
}
