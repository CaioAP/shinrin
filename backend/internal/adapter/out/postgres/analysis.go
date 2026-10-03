package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

var (
	_ port.ScoreReader  = (*Store)(nil)
	_ port.ScoreWriter  = (*Store)(nil)
	_ port.ReportWriter = (*Store)(nil)
)

// scoreDetails is the jsonb shape of scores.details.
type scoreDetails struct {
	PeerGroup string      `json:"peer_group,omitempty"`
	Inputs    []inputJSON `json:"inputs"`
}

type inputJSON struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
	Points float64 `json:"points"`
	Weight float64 `json:"weight"`
	Peers  int     `json:"peers,omitempty"`
}

// UpsertScorecards implements port.ScoreWriter. A scorecard replaces every
// factor stored for its asset and date, so a factor that lost its data does
// not linger.
func (s *Store) UpsertScorecards(ctx context.Context, cards []domain.Scorecard) error {
	if len(cards) == 0 {
		return nil
	}
	var keyMarket, keySymbol []string
	var keyAsOf []time.Time
	var market, symbol, factor []string
	var asOf []time.Time
	var value []float64
	var details [][]byte
	for _, c := range cards {
		keyMarket = append(keyMarket, string(c.Asset.Market))
		keySymbol = append(keySymbol, string(c.Asset.Symbol))
		keyAsOf = append(keyAsOf, c.AsOf)
		for _, f := range c.Factors {
			d := scoreDetails{PeerGroup: f.PeerGroup, Inputs: make([]inputJSON, len(f.Inputs))}
			for i, in := range f.Inputs {
				d.Inputs[i] = inputJSON{in.Metric, in.Value, in.Points, in.Weight, in.Peers}
			}
			b, err := json.Marshal(d)
			if err != nil {
				return err
			}
			market = append(market, string(c.Asset.Market))
			symbol = append(symbol, string(c.Asset.Symbol))
			asOf = append(asOf, c.AsOf)
			factor = append(factor, string(f.Factor))
			value = append(value, f.Value)
			details = append(details, b)
		}
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			DELETE FROM scores sc USING assets a,
				unnest($1::text[], $2::text[], $3::date[]) AS k(market, symbol, as_of)
			WHERE a.id = sc.asset_id AND a.market = k.market AND a.symbol = k.symbol AND sc.as_of = k.as_of`,
			keyMarket, keySymbol, keyAsOf); err != nil {
			return fmt.Errorf("clear scores: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO scores (asset_id, as_of, factor, value, details)
			SELECT DISTINCT ON (a.id, x.as_of, x.factor) a.id, x.as_of, x.factor, x.value, x.details
			FROM unnest($1::text[], $2::text[], $3::date[], $4::text[], $5::float8[], $6::jsonb[])
				AS x(market, symbol, as_of, factor, value, details)
			JOIN assets a ON a.market = x.market AND a.symbol = x.symbol`,
			market, symbol, asOf, factor, value, details); err != nil {
			return fmt.Errorf("insert scores: %w", err)
		}
		return nil
	})
}

// LatestScorecard implements port.ScoreReader.
func (s *Store) LatestScorecard(ctx context.Context, asset domain.AssetKey) (domain.Scorecard, error) {
	cards, err := s.scorecards(ctx, `a.market = $1 AND a.symbol = $2`, asset.Market, asset.Symbol)
	if err != nil {
		return domain.Scorecard{}, err
	}
	if len(cards) == 0 {
		return domain.Scorecard{}, domain.ErrNotFound
	}
	return cards[0], nil
}

// LatestScorecards implements port.ScoreReader.
func (s *Store) LatestScorecards(ctx context.Context, market domain.Market) ([]domain.Scorecard, error) {
	return s.scorecards(ctx, `a.market = $1`, market)
}

// scorecards reads each matching asset's newest scorecard, ordered by
// asset.
func (s *Store) scorecards(ctx context.Context, where string, args ...any) ([]domain.Scorecard, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.market, a.symbol, sc.as_of, sc.factor, sc.value, sc.details
		FROM scores sc JOIN assets a ON a.id = sc.asset_id
		WHERE `+where+`
		  AND sc.as_of = (SELECT max(as_of) FROM scores WHERE asset_id = a.id)
		ORDER BY a.market, a.symbol, sc.factor`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Scorecard
	for rows.Next() {
		var (
			key     domain.AssetKey
			asOf    time.Time
			fs      domain.FactorScore
			details []byte
		)
		if err := rows.Scan(&key.Market, &key.Symbol, &asOf, &fs.Factor, &fs.Value, &details); err != nil {
			return nil, err
		}
		var d scoreDetails
		if err := json.Unmarshal(details, &d); err != nil {
			return nil, fmt.Errorf("score details of %s: %w", key, err)
		}
		fs.PeerGroup = d.PeerGroup
		for _, in := range d.Inputs {
			fs.Inputs = append(fs.Inputs, domain.ScoreInput{Metric: in.Metric, Value: in.Value, Points: in.Points, Weight: in.Weight, Peers: in.Peers})
		}
		if n := len(out); n == 0 || out[n-1].Asset != key {
			out = append(out, domain.Scorecard{Asset: key, AsOf: asOf})
		}
		out[len(out)-1].Factors = append(out[len(out)-1].Factors, fs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		sortFactors(out[i].Factors)
	}
	return out, nil
}

// sortFactors puts factors in domain.Factors order (SQL sorted them by
// name).
func sortFactors(fs []domain.FactorScore) {
	slices.SortStableFunc(fs, func(a, b domain.FactorScore) int {
		return slices.Index(domain.Factors, a.Factor) - slices.Index(domain.Factors, b.Factor)
	})
}

// snapshotJSON and outputJSON are the jsonb shapes of a report's input
// snapshot and output; the output matches the design's report schema.
type snapshotJSON struct {
	Facts     map[string]float64 `json:"facts"`
	Labels    map[string]string  `json:"labels"`
	Headlines []headlineJSON     `json:"headlines"`
}

type headlineJSON struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Source      string    `json:"source"`
	PublishedAt time.Time `json:"published_at"`
}

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

// SaveReport implements port.ReportWriter.
func (s *Store) SaveReport(ctx context.Context, r domain.Report) (int64, error) {
	snap := snapshotJSON{Facts: r.Snapshot.Facts, Labels: r.Snapshot.Labels, Headlines: []headlineJSON{}}
	for _, h := range r.Snapshot.Headlines {
		snap.Headlines = append(snap.Headlines, headlineJSON{h.ID, h.Title, h.Source, h.PublishedAt})
	}
	o := r.Output
	out := outputJSON{Summary: o.Summary, BullCase: o.BullCase, BearCase: o.BearCase, ValuationView: o.ValuationView,
		TimingView: o.TimingView, FitForProfile: o.FitForProfile, KeyRisks: o.KeyRisks, Confidence: o.Confidence, CitedData: o.CitedData}
	out.Allocation.Min, out.Allocation.Max = o.AllocationMinPct, o.AllocationMaxPct
	snapJSON, err := json.Marshal(snap)
	if err != nil {
		return 0, err
	}
	outJSON, err := json.Marshal(out)
	if err != nil {
		return 0, err
	}
	omitted := r.Omitted
	if omitted == nil {
		omitted = []string{}
	}
	var id int64
	err = s.pool.QueryRow(ctx, `
		INSERT INTO ai_reports (asset_id, kind, profile, as_of, input_snapshot, output, omitted, provider, model, tokens_in, tokens_out)
		VALUES ((SELECT id FROM assets WHERE market = $1 AND symbol = $2), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`,
		r.Asset.Market, r.Asset.Symbol, r.Kind, r.Profile, r.AsOf, snapJSON, outJSON, omitted, r.Provider, r.Model, r.TokensIn, r.TokensOut).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("save report: %w", err)
	}
	return id, nil
}
