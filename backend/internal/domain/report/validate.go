package report

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Section names, as in the report JSON.
const (
	SectionSummary    = "summary"
	SectionBullCase   = "bull_case"
	SectionBearCase   = "bear_case"
	SectionValuation  = "valuation_view"
	SectionTiming     = "timing_view"
	SectionFit        = "fit_for_profile"
	SectionAllocation = "suggested_allocation_pct"
	SectionKeyRisks   = "key_risks"
	SectionConfidence = "confidence"
	SectionCitedData  = "cited_data"
)

// Issue is one validation failure. Item is the index within a list section,
// or -1 for the whole section.
type Issue struct {
	Section string
	Item    int
	Message string
}

func (i Issue) String() string {
	if i.Item >= 0 {
		return fmt.Sprintf("%s[%d]: %s", i.Section, i.Item, i.Message)
	}
	return i.Section + ": " + i.Message
}

var enums = map[string][]string{
	SectionValuation:  {"cheap", "fair", "expensive"},
	SectionTiming:     {"accumulate", "wait", "avoid"},
	SectionFit:        {"good", "partial", "poor"},
	SectionConfidence: {"low", "medium", "high"},
}

// Validate checks a report against its snapshot: enum fields hold allowed
// values, the allocation stays inside the profile's band, every citation
// names snapshot data, and every number in the prose matches a snapshot
// number (allowing for rounding, percentages and magnitudes like "bn").
func Validate(out domain.ReportOutput, s domain.ReportSnapshot) []Issue {
	var issues []Issue
	allowed := allowedNumbers(s, out)

	if bad := unsupported(out.Summary, allowed); len(bad) > 0 {
		issues = append(issues, Issue{SectionSummary, -1, "numbers not in the data: " + strings.Join(bad, ", ")})
	}
	if strings.TrimSpace(out.Summary) == "" {
		issues = append(issues, Issue{SectionSummary, -1, "empty"})
	}
	for section, items := range map[string][]string{SectionBullCase: out.BullCase, SectionBearCase: out.BearCase, SectionKeyRisks: out.KeyRisks} {
		if len(items) == 0 {
			issues = append(issues, Issue{section, -1, "empty: give at least one point"})
		}
		for i, it := range items {
			if bad := unsupported(it, allowed); len(bad) > 0 {
				issues = append(issues, Issue{section, i, "numbers not in the data: " + strings.Join(bad, ", ")})
			}
		}
	}
	for section, v := range map[string]string{SectionValuation: out.ValuationView, SectionTiming: out.TimingView, SectionFit: out.FitForProfile, SectionConfidence: out.Confidence} {
		if !contains(enums[section], v) {
			issues = append(issues, Issue{section, -1, fmt.Sprintf("%q is not one of %s", v, strings.Join(enums[section], ", "))})
		}
	}

	maxPct := 100.0
	if v, ok := s.Facts["band_max_pct"]; ok {
		maxPct = v
	}
	if out.AllocationMinPct < 0 || out.AllocationMinPct > out.AllocationMaxPct || out.AllocationMaxPct > maxPct {
		issues = append(issues, Issue{SectionAllocation, -1, fmt.Sprintf("min %.1f and max %.1f must satisfy 0 <= min <= max <= %.1f (the profile's band for this class)", out.AllocationMinPct, out.AllocationMaxPct, maxPct)})
	}

	if len(out.CitedData) == 0 {
		issues = append(issues, Issue{SectionCitedData, -1, "cite the data keys the report relies on"})
	}
	for i, c := range out.CitedData {
		if !s.Has(c) {
			issues = append(issues, Issue{SectionCitedData, i, fmt.Sprintf("%q is not a key in the data", c)})
		}
	}
	return issues
}

// Omit removes what failed validation: failing list items, unknown
// citations, and whole scalar sections (blanked). It returns the cleaned
// output and the names of what was removed, so the UI can say so.
func Omit(out domain.ReportOutput, issues []Issue) (domain.ReportOutput, []string) {
	drop := map[string]map[int]bool{}
	var omitted []string
	for _, is := range issues {
		if drop[is.Section] == nil {
			drop[is.Section] = map[int]bool{}
		}
		drop[is.Section][is.Item] = true
	}
	filter := func(section string, items []string) []string {
		d := drop[section]
		if d == nil {
			return items
		}
		if d[-1] && len(items) > 0 {
			omitted = append(omitted, section)
			return nil
		}
		var kept []string
		for i, it := range items {
			if d[i] {
				omitted = append(omitted, fmt.Sprintf("%s[%d]", section, i))
				continue
			}
			kept = append(kept, it)
		}
		return kept
	}
	out.BullCase = filter(SectionBullCase, out.BullCase)
	out.BearCase = filter(SectionBearCase, out.BearCase)
	out.KeyRisks = filter(SectionKeyRisks, out.KeyRisks)
	out.CitedData = filter(SectionCitedData, out.CitedData)

	blank := func(section string, f func()) {
		if drop[section] != nil {
			f()
			omitted = append(omitted, section)
		}
	}
	blank(SectionSummary, func() { out.Summary = "" })
	blank(SectionValuation, func() { out.ValuationView = "" })
	blank(SectionTiming, func() { out.TimingView = "" })
	blank(SectionFit, func() { out.FitForProfile = "" })
	blank(SectionConfidence, func() { out.Confidence = "" })
	blank(SectionAllocation, func() { out.AllocationMinPct, out.AllocationMaxPct = 0, 0 })
	return out, omitted
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// number is a figure found in prose. unit is its magnitude multiplier.
type number struct {
	raw      string
	value    float64
	decimals int
	unit     float64
}

var units = map[string]float64{
	"%": 1, "x": 1, "pp": 1, "bps": 1,
	"k": 1e3, "thousand": 1e3, "mil": 1e3,
	"m": 1e6, "mm": 1e6, "mn": 1e6, "million": 1e6, "millions": 1e6, "milhão": 1e6, "milhões": 1e6, "mi": 1e6,
	"b": 1e9, "bn": 1e9, "billion": 1e9, "billions": 1e9, "bilhão": 1e9, "bilhões": 1e9, "bi": 1e9,
	"t": 1e12, "tn": 1e12, "trillion": 1e12, "tri": 1e12, "trilhão": 1e12, "trilhões": 1e12,
}

// Phrases where a small number is part of a standard term ("200-day
// average", "52-week high", "10-year yield"), not a claim.
var periodWords = []string{"day", "week", "month", "year", "quarter", "dia", "semana", "mes", "mês", "meses", "ano", "trimestre"}

// numbersIn extracts the figures in text. Digits glued to letters (PETR4,
// Q3, 3Q25) are identifiers, not figures.
func numbersIn(text string) []number {
	r := []rune(text)
	var out []number
	for i := 0; i < len(r); i++ {
		if !unicode.IsDigit(r[i]) {
			continue
		}
		if i > 0 && (unicode.IsLetter(r[i-1]) || r[i-1] == '_' || r[i-1] == ':') {
			for i < len(r) && (unicode.IsLetter(r[i]) || unicode.IsDigit(r[i])) {
				i++
			}
			continue
		}
		start, j := i, i
		for j < len(r) && (unicode.IsDigit(r[j]) || ((r[j] == '.' || r[j] == ',') && j+1 < len(r) && unicode.IsDigit(r[j+1]))) {
			j++
		}
		raw := string(r[i:j])
		// Unit: % directly, or a word after optional spaces.
		k := j
		for k < len(r) && r[k] == ' ' {
			k++
		}
		w := k
		for w < len(r) && (unicode.IsLetter(r[w]) || r[w] == '%') {
			w++
		}
		word := strings.ToLower(string(r[k:w]))
		unit, isUnit := units[word]
		if !isUnit {
			unit = 1
			if k == j && w > k { // letters glued to the digits: an identifier
				i = w
				continue
			}
		}
		rest := strings.ToLower(string(r[j:min(len(r), j+12)]))
		periodic := false
		for _, p := range periodWords {
			if strings.HasPrefix(strings.TrimLeft(rest, " -"), p) {
				periodic = true
			}
		}
		i = j
		if periodic {
			continue
		}
		v, dec, ok := parseNumber(raw)
		if !ok {
			continue
		}
		end := j
		if isUnit {
			end = w
		}
		out = append(out, number{raw: string(r[start:end]), value: v, decimals: dec, unit: unit})
		if isUnit {
			i = w
		}
	}
	return out
}

// parseNumber reads "1,234.5" (English), "12,5" (Portuguese decimal) and
// plain numbers, returning the value and its decimal places.
func parseNumber(s string) (float64, int, bool) {
	switch {
	case strings.Contains(s, ".") && strings.Contains(s, ","):
		if strings.LastIndex(s, ".") > strings.LastIndex(s, ",") {
			s = strings.ReplaceAll(s, ",", "")
		} else { // 1.234,5
			s = strings.ReplaceAll(strings.ReplaceAll(s, ".", ""), ",", ".")
		}
	case strings.Contains(s, ","):
		parts := strings.Split(s, ",")
		thousands := len(parts) > 1
		for _, p := range parts[1:] {
			thousands = thousands && len(p) == 3
		}
		if thousands {
			s = strings.ReplaceAll(s, ",", "")
		} else {
			s = strings.ReplaceAll(s, ",", ".")
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, 0, false
	}
	dec := 0
	if i := strings.IndexByte(s, '.'); i >= 0 {
		dec = len(s) - i - 1
	}
	return v, dec, true
}

// allowedNumbers is every figure the prose may quote: snapshot facts (also
// as percentages), figures already in the snapshot's labels (thresholds in
// signal messages), and the report's own allocation.
func allowedNumbers(s domain.ReportSnapshot, out domain.ReportOutput) []float64 {
	var vals []float64
	for _, v := range s.Facts {
		vals = append(vals, math.Abs(v), math.Abs(v*100))
	}
	for _, l := range s.Labels {
		for _, n := range numbersIn(l) {
			vals = append(vals, n.value*n.unit)
		}
	}
	for _, h := range s.Headlines {
		for _, n := range numbersIn(h.Title) {
			vals = append(vals, n.value*n.unit)
		}
	}
	return append(vals, out.AllocationMinPct, out.AllocationMaxPct)
}

// unsupported returns the figures in text that match no allowed number.
// Small whole numbers (counts, "two risks") and years are let through.
func unsupported(text string, allowed []float64) []string {
	var bad []string
	for _, n := range numbersIn(text) {
		v := n.value * n.unit
		if n.decimals == 0 && n.unit == 1 && (n.value <= 12 || (n.value >= 1900 && n.value <= 2100)) && !strings.HasSuffix(n.raw, "%") {
			continue
		}
		tol := 0.5*math.Pow(10, -float64(n.decimals))*n.unit + 1e-9
		ok := false
		for _, a := range allowed {
			if math.Abs(v-a) <= max(tol, 0.01*a) {
				ok = true
				break
			}
		}
		if !ok {
			bad = append(bad, n.raw)
		}
	}
	return bad
}
