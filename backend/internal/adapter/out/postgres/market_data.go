package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Writes send whole batches as arrays and join them to assets by
// (market, symbol), so one round trip stores thousands of rows and rows for
// unknown assets are dropped by the join. DISTINCT ON keeps one row per key,
// because ON CONFLICT cannot touch the same row twice in one statement.

// UpsertPrices implements port.PriceWriter.
func (s *Store) UpsertPrices(ctx context.Context, bars []domain.PriceBar) error {
	if len(bars) == 0 {
		return nil
	}
	n := len(bars)
	market, symbol, source := make([]string, n), make([]string, n), make([]string, n)
	date := make([]time.Time, n)
	open, high, low, closes, adj := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	volume := make([]int64, n)
	for i, b := range bars {
		market[i], symbol[i], source[i] = string(b.Asset.Market), string(b.Asset.Symbol), b.Source
		date[i] = b.Date
		open[i], high[i], low[i], closes[i], adj[i] = b.Open, b.High, b.Low, b.Close, b.AdjClose
		volume[i] = b.Volume
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO prices_daily (asset_id, date, open, high, low, close, adj_close, volume, source)
		SELECT DISTINCT ON (a.id, x.date) a.id, x.date, x.open, x.high, x.low, x.close, NULLIF(x.adj, 0), x.volume, x.source
		FROM unnest($1::text[], $2::text[], $3::date[], $4::float8[], $5::float8[], $6::float8[], $7::float8[], $8::float8[], $9::int8[], $10::text[])
			AS x(market, symbol, date, open, high, low, close, adj, volume, source)
		JOIN assets a ON a.market = x.market AND a.symbol = x.symbol
		ON CONFLICT (asset_id, date) DO UPDATE SET
			open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low, close = EXCLUDED.close,
			adj_close = EXCLUDED.adj_close, volume = EXCLUDED.volume, source = EXCLUDED.source, fetched_at = now()`,
		market, symbol, date, open, high, low, closes, adj, volume, source)
	if err != nil {
		return fmt.Errorf("upsert prices: %w", err)
	}
	return nil
}

// PricesBetween implements port.PriceReader.
func (s *Store) PricesBetween(ctx context.Context, asset domain.AssetKey, from, to time.Time) ([]domain.PriceBar, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.date, p.open, p.high, p.low, p.close, COALESCE(p.adj_close, 0), p.volume, p.source
		FROM prices_daily p JOIN assets a ON a.id = p.asset_id
		WHERE a.market = $1 AND a.symbol = $2 AND p.date BETWEEN $3::date AND $4::date
		ORDER BY p.date`, asset.Market, asset.Symbol, from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.PriceBar, error) {
		b := domain.PriceBar{Asset: asset}
		err := r.Scan(&b.Date, &b.Open, &b.High, &b.Low, &b.Close, &b.AdjClose, &b.Volume, &b.Source)
		return b, err
	})
}

// LatestPriceDate implements port.PriceReader.
func (s *Store) LatestPriceDate(ctx context.Context, asset domain.AssetKey) (time.Time, bool, error) {
	return s.latestDate(ctx, `SELECT max(p.date) FROM prices_daily p JOIN assets a ON a.id = p.asset_id WHERE a.market = $1 AND a.symbol = $2`, asset)
}

func (s *Store) latestDate(ctx context.Context, q string, asset domain.AssetKey) (time.Time, bool, error) {
	var d *time.Time
	if err := s.pool.QueryRow(ctx, q, asset.Market, asset.Symbol).Scan(&d); err != nil {
		return time.Time{}, false, err
	}
	if d == nil {
		return time.Time{}, false, nil
	}
	return *d, true, nil
}

// UpsertFundamentals implements port.FundamentalWriter.
func (s *Store) UpsertFundamentals(ctx context.Context, rows []domain.Fundamental) error {
	if len(rows) == 0 {
		return nil
	}
	n := len(rows)
	market, symbol, ptype, metric, source := make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	end := make([]time.Time, n)
	value := make([]float64, n)
	for i, r := range rows {
		market[i], symbol[i], ptype[i], metric[i], source[i] = string(r.Asset.Market), string(r.Asset.Symbol), string(r.PeriodType), r.Metric, r.Source
		end[i], value[i] = r.PeriodEnd, r.Value
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO fundamentals (asset_id, period_end, period_type, metric, value, source)
		SELECT DISTINCT ON (a.id, x.period_end, x.period_type, x.metric) a.id, x.period_end, x.period_type, x.metric, x.value, x.source
		FROM unnest($1::text[], $2::text[], $3::date[], $4::text[], $5::text[], $6::float8[], $7::text[])
			AS x(market, symbol, period_end, period_type, metric, value, source)
		JOIN assets a ON a.market = x.market AND a.symbol = x.symbol
		ON CONFLICT (asset_id, period_end, period_type, metric) DO UPDATE SET
			value = EXCLUDED.value, source = EXCLUDED.source, fetched_at = now()`,
		market, symbol, end, ptype, metric, value, source)
	if err != nil {
		return fmt.Errorf("upsert fundamentals: %w", err)
	}
	return nil
}

// FundamentalsSince implements port.FundamentalReader.
func (s *Store) FundamentalsSince(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.Fundamental, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT f.period_end, f.period_type, f.metric, f.value, f.source
		FROM fundamentals f JOIN assets a ON a.id = f.asset_id
		WHERE a.market = $1 AND a.symbol = $2 AND f.period_end >= $3::date
		ORDER BY f.period_end, f.metric, f.period_type`, asset.Market, asset.Symbol, since)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Fundamental, error) {
		f := domain.Fundamental{Asset: asset}
		err := r.Scan(&f.PeriodEnd, &f.PeriodType, &f.Metric, &f.Value, &f.Source)
		return f, err
	})
}

// LatestPeriodEnd implements port.FundamentalReader.
func (s *Store) LatestPeriodEnd(ctx context.Context, asset domain.AssetKey) (time.Time, bool, error) {
	return s.latestDate(ctx, `SELECT max(f.period_end) FROM fundamentals f JOIN assets a ON a.id = f.asset_id WHERE a.market = $1 AND a.symbol = $2`, asset)
}

// UpsertCorporateActions implements port.CorporateActionWriter.
func (s *Store) UpsertCorporateActions(ctx context.Context, actions []domain.CorporateAction) error {
	if len(actions) == 0 {
		return nil
	}
	n := len(actions)
	market, symbol, typ, source := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	ex := make([]time.Time, n)
	value := make([]float64, n)
	for i, a := range actions {
		market[i], symbol[i], typ[i], source[i] = string(a.Asset.Market), string(a.Asset.Symbol), string(a.Type), a.Source
		ex[i], value[i] = a.ExDate, a.Value
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO corporate_actions (asset_id, ex_date, type, value, source)
		SELECT DISTINCT ON (a.id, x.ex_date, x.type) a.id, x.ex_date, x.type, x.value, x.source
		FROM unnest($1::text[], $2::text[], $3::date[], $4::text[], $5::float8[], $6::text[])
			AS x(market, symbol, ex_date, type, value, source)
		JOIN assets a ON a.market = x.market AND a.symbol = x.symbol
		ON CONFLICT (asset_id, ex_date, type) DO UPDATE SET
			value = EXCLUDED.value, source = EXCLUDED.source, fetched_at = now()`,
		market, symbol, ex, typ, value, source)
	if err != nil {
		return fmt.Errorf("upsert corporate actions: %w", err)
	}
	return nil
}

// CorporateActionsSince implements port.CorporateActionReader.
func (s *Store) CorporateActionsSince(ctx context.Context, asset domain.AssetKey, since time.Time) ([]domain.CorporateAction, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.ex_date, c.type, c.value, c.source
		FROM corporate_actions c JOIN assets a ON a.id = c.asset_id
		WHERE a.market = $1 AND a.symbol = $2 AND c.ex_date >= $3::date
		ORDER BY c.ex_date, c.type`, asset.Market, asset.Symbol, since)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.CorporateAction, error) {
		c := domain.CorporateAction{Asset: asset}
		err := r.Scan(&c.ExDate, &c.Type, &c.Value, &c.Source)
		return c, err
	})
}

// UpsertIndicators implements port.IndicatorWriter. A set replaces every
// indicator stored for its asset and date, so a ratio that stopped being
// meaningful (P/E after a loss) does not linger.
func (s *Store) UpsertIndicators(ctx context.Context, sets []domain.IndicatorSet) error {
	if len(sets) == 0 {
		return nil
	}
	var market, symbol, name []string
	var asOf []time.Time
	var value []float64
	var keyMarket, keySymbol []string
	var keyAsOf []time.Time
	for _, set := range sets {
		keyMarket = append(keyMarket, string(set.Asset.Market))
		keySymbol = append(keySymbol, string(set.Asset.Symbol))
		keyAsOf = append(keyAsOf, set.AsOf)
		for k, v := range set.Values {
			market = append(market, string(set.Asset.Market))
			symbol = append(symbol, string(set.Asset.Symbol))
			asOf = append(asOf, set.AsOf)
			name = append(name, k)
			value = append(value, v)
		}
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			DELETE FROM indicators i USING assets a,
				unnest($1::text[], $2::text[], $3::date[]) AS k(market, symbol, as_of)
			WHERE a.id = i.asset_id AND a.market = k.market AND a.symbol = k.symbol AND i.as_of = k.as_of`,
			keyMarket, keySymbol, keyAsOf); err != nil {
			return fmt.Errorf("clear indicators: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO indicators (asset_id, as_of, name, value)
			SELECT DISTINCT ON (a.id, x.as_of, x.name) a.id, x.as_of, x.name, x.value
			FROM unnest($1::text[], $2::text[], $3::date[], $4::text[], $5::float8[]) AS x(market, symbol, as_of, name, value)
			JOIN assets a ON a.market = x.market AND a.symbol = x.symbol`,
			market, symbol, asOf, name, value); err != nil {
			return fmt.Errorf("insert indicators: %w", err)
		}
		return nil
	})
}

// LatestIndicators implements port.IndicatorReader.
func (s *Store) LatestIndicators(ctx context.Context, asset domain.AssetKey) (domain.IndicatorSet, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT i.as_of, i.name, i.value
		FROM indicators i JOIN assets a ON a.id = i.asset_id
		WHERE a.market = $1 AND a.symbol = $2
		  AND i.as_of = (SELECT max(as_of) FROM indicators WHERE asset_id = a.id)`, asset.Market, asset.Symbol)
	if err != nil {
		return domain.IndicatorSet{}, err
	}
	defer rows.Close()
	set := domain.IndicatorSet{Asset: asset, Values: map[string]float64{}}
	for rows.Next() {
		var name string
		var v float64
		if err := rows.Scan(&set.AsOf, &name, &v); err != nil {
			return domain.IndicatorSet{}, err
		}
		set.Values[name] = v
	}
	if err := rows.Err(); err != nil {
		return domain.IndicatorSet{}, err
	}
	if len(set.Values) == 0 {
		return domain.IndicatorSet{}, domain.ErrNotFound
	}
	return set, nil
}
