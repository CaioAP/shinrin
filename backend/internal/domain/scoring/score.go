package scoring

import (
	"fmt"
	"math"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Input is one asset's metrics as of a date, ready to be scored.
type Input struct {
	Asset   domain.Asset
	AsOf    time.Time
	Metrics map[string]float64
}

// Peer group sizes. A sector with fewer members than MinSectorPeers is too
// small to rank within, so the asset is compared with its whole market and
// class; a metric needs MinMetricPeers values to be ranked at all.
const (
	MinSectorPeers = 5
	MinMetricPeers = 3
)

type groupKey struct {
	market domain.Market
	model  string
	sector string // empty for the market-wide group
}

// Score computes every input's scorecard. Peer-relative metrics are ranked
// against assets of the same market, model (stock, financials, fund) and,
// when it is large enough, sector. Assets whose class has no model are
// skipped.
func Score(inputs []Input) []domain.Scorecard {
	type entry struct {
		in    Input
		model Model
	}
	var entries []entry
	groups := map[groupKey][]map[string]float64{}
	for _, in := range inputs {
		m, ok := ModelFor(in.Asset)
		if !ok {
			continue
		}
		entries = append(entries, entry{in, m})
		wide := groupKey{in.Asset.Key.Market, m.Name, ""}
		groups[wide] = append(groups[wide], in.Metrics)
		if in.Asset.Sector != "" {
			sk := groupKey{in.Asset.Key.Market, m.Name, in.Asset.Sector}
			groups[sk] = append(groups[sk], in.Metrics)
		}
	}

	out := make([]domain.Scorecard, 0, len(entries))
	for _, e := range entries {
		wide := groupKey{e.in.Asset.Key.Market, e.model.Name, ""}
		primary, label := wide, fmt.Sprintf("%s %s (market-wide)", wide.market, wide.model)
		if s := e.in.Asset.Sector; s != "" && len(groups[groupKey{wide.market, wide.model, s}]) >= MinSectorPeers {
			primary = groupKey{wide.market, wide.model, s}
			label = fmt.Sprintf("%s %s, sector %s", wide.market, wide.model, s)
		}
		sc := domain.Scorecard{Asset: e.in.Asset.Key, AsOf: e.in.AsOf}
		for _, f := range domain.Factors {
			specs, ok := e.model.Factors[f]
			if !ok {
				continue
			}
			if fs, ok := scoreFactor(f, specs, e.in.Metrics, groups[primary], groups[wide]); ok {
				fs.PeerGroup = label
				sc.Factors = append(sc.Factors, fs)
			}
		}
		out = append(out, sc)
	}
	return out
}

func scoreFactor(f domain.Factor, specs []MetricSpec, own map[string]float64, primary, wide []map[string]float64) (domain.FactorScore, bool) {
	fs := domain.FactorScore{Factor: f}
	var sum, weights float64
	for _, spec := range specs {
		v, ok := own[spec.Name]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		in := domain.ScoreInput{Metric: spec.Name, Value: v, Weight: spec.Weight}
		if spec.Absolute != nil {
			in.Points = clamp(spec.Absolute(v))
		} else {
			peers := values(primary, spec.Name)
			if len(peers) < MinMetricPeers {
				peers = values(wide, spec.Name)
			}
			if len(peers) < MinMetricPeers {
				continue
			}
			in.Points = Percentile(peers, v)
			if spec.Direction == LowerIsBetter {
				in.Points = 100 - in.Points
			}
			in.Peers = len(peers)
		}
		in.Points = round1(in.Points)
		fs.Inputs = append(fs.Inputs, in)
		sum += in.Points * spec.Weight
		weights += spec.Weight
	}
	if weights == 0 {
		return fs, false
	}
	fs.Value = round1(sum / weights)
	return fs, true
}

func values(group []map[string]float64, metric string) []float64 {
	var out []float64
	for _, m := range group {
		if v, ok := m[metric]; ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
			out = append(out, v)
		}
	}
	return out
}

// Percentile ranks v within values (which should include v) from 0 (lowest)
// to 100 (highest). Ties share their mid rank, so equal values score the
// same and a group of identical values all score 50.
func Percentile(values []float64, v float64) float64 {
	if len(values) < 2 {
		return 50
	}
	var less, equal int
	for _, x := range values {
		switch {
		case x < v:
			less++
		case x == v:
			equal++
		}
	}
	if equal == 0 { // v is not one of values: rank it as an outsider
		return 100 * float64(less) / float64(len(values))
	}
	return 100 * (float64(less) + 0.5*float64(equal-1)) / float64(len(values)-1)
}

func clamp(v float64) float64 { return max(0, min(100, v)) }

func round1(v float64) float64 { return math.Round(v*10) / 10 }
