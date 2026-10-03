package report

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/report"
)

// systemPrompt sets the analyst role and the grounding rules the validator
// enforces afterwards.
const systemPrompt = `You are an equity research analyst writing a short, balanced note on one investment for one private investor, in the style of a sell-side research note.

Shinrin, the app you work inside, has already computed every number with a deterministic engine: factor scores from 0 to 100 against sector peers (higher is always better for the investor: a high valuation score means cheap, a high risk score means low risk), valuation and quality ratios, technical indicators, fair value ranges, the engine's own valuation and timing views with the signals behind them, macro context, the investor's risk profile and the allocation band that profile allows for this asset class, and recent headlines. Your job is judgment and explanation on top of those numbers.

Rules:
- Use only the data provided. Every figure you write must appear in the data. You may round it and may write a fraction as a percentage (0.2234 as 22.3%), but never compute new figures: no price targets, no upside percentages, no forecasts, no numbers from memory.
- Ratios in the data are fractions unless the key ends in _pct or is a macro rate in percent (selic, ust_10y, ipca_12m, real_rate_br).
- In cited_data list the keys you relied on: fact or label keys exactly as given, or headline ids such as news:1.
- Present both sides: at least one bull case point and one bear case point, each tied to data.
- valuation_view and timing_view are your own judgment; the engine's views are in the data, and if you disagree, say why in the summary.
- suggested_allocation_pct is a position size range in percent of the whole portfolio, between 0 and band_max_pct (the profile's band for this asset class). Use 0 to 0 when the asset does not fit the profile.
- Missing data is information: if a factor is not scored or a note says something could not be computed, say what that leaves uncertain and lower your confidence.
- This is educational analysis, not financial advice. Do not tell the reader to buy or sell; describe what the data suggests for an investor with this profile.
- Write numbers with a dot as the decimal separator and no thousands separators.`

// schema is the JSON shape the design fixes for a report (docs/design.md,
// section 9), sent as structured output so the provider enforces it.
var schema = mustJSON(map[string]any{
	"type": "object",
	"properties": map[string]any{
		"summary":         map[string]any{"type": "string"},
		"bull_case":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"bear_case":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"valuation_view":  map[string]any{"type": "string", "enum": []string{"cheap", "fair", "expensive"}},
		"timing_view":     map[string]any{"type": "string", "enum": []string{"accumulate", "wait", "avoid"}},
		"fit_for_profile": map[string]any{"type": "string", "enum": []string{"good", "partial", "poor"}},
		"suggested_allocation_pct": map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"min": map[string]any{"type": "number"}, "max": map[string]any{"type": "number"}},
			"required":             []string{"min", "max"},
			"additionalProperties": false,
		},
		"key_risks":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"confidence": map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
		"cited_data": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	},
	"required":             []string{"summary", "bull_case", "bear_case", "valuation_view", "timing_view", "fit_for_profile", "suggested_allocation_pct", "key_risks", "confidence", "cited_data"},
	"additionalProperties": false,
})

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// outputJSON mirrors schema.
type outputJSON struct {
	Summary       string   `json:"summary"`
	BullCase      []string `json:"bull_case"`
	BearCase      []string `json:"bear_case"`
	ValuationView string   `json:"valuation_view"`
	TimingView    string   `json:"timing_view"`
	FitForProfile string   `json:"fit_for_profile"`
	Allocation    struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	} `json:"suggested_allocation_pct"`
	KeyRisks   []string `json:"key_risks"`
	Confidence string   `json:"confidence"`
	CitedData  []string `json:"cited_data"`
}

// parseOutput reads the model's JSON, tolerating a Markdown code fence from
// providers without strict structured output.
func parseOutput(text string) (domain.ReportOutput, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(strings.TrimPrefix(text, "```json"), "```")
		text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	}
	var o outputJSON
	if err := json.Unmarshal([]byte(text), &o); err != nil {
		return domain.ReportOutput{}, fmt.Errorf("%w: report is not valid JSON: %v", domain.ErrInvalid, err)
	}
	return domain.ReportOutput{
		Summary: o.Summary, BullCase: o.BullCase, BearCase: o.BearCase,
		ValuationView: o.ValuationView, TimingView: o.TimingView, FitForProfile: o.FitForProfile,
		AllocationMinPct: o.Allocation.Min, AllocationMaxPct: o.Allocation.Max,
		KeyRisks: o.KeyRisks, Confidence: o.Confidence, CitedData: o.CitedData,
	}, nil
}

// userPrompt renders the snapshot as the data block. JSON keeps keys exact
// for citation; map keys are emitted sorted, so the same snapshot always
// produces the same prompt.
func userPrompt(s domain.ReportSnapshot, lang string) string {
	type headline struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Source    string `json:"source"`
		Published string `json:"published"`
	}
	hs := make([]headline, 0, len(s.Headlines))
	for _, h := range s.Headlines {
		hs = append(hs, headline{h.ID, h.Title, h.Source, h.PublishedAt.Format("2006-01-02")})
	}
	data, _ := json.MarshalIndent(map[string]any{"facts": s.Facts, "labels": s.Labels, "headlines": hs}, "", "  ")
	language := "English"
	if strings.HasPrefix(strings.ToLower(lang), "pt") {
		language = "Brazilian Portuguese"
	}
	return fmt.Sprintf("Write the research note for %s (%s) for a %s investor. Write the prose in %s; keep enum values and data keys in English.\n\nDATA\n%s",
		s.Labels["symbol"], s.Labels["name"], s.Labels["risk_profile"], language, data)
}

// retryPrompt asks the model to fix its previous answer.
func retryPrompt(first string, previous string, problems []string) string {
	return first + "\n\nYour previous answer was:\n" + previous +
		"\n\nIt failed these checks:\n- " + strings.Join(problems, "\n- ") +
		"\n\nReturn the full corrected note. Remove any figure that is not in the data rather than guessing."
}

func issueStrings(issues []report.Issue) []string {
	out := make([]string, len(issues))
	for i, is := range issues {
		out[i] = is.String()
	}
	slices.Sort(out)
	return out
}
