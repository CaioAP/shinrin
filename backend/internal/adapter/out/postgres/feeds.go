package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

var (
	_ port.QuoteWriter     = (*Store)(nil)
	_ port.QuoteReader     = (*Store)(nil)
	_ port.NewsWriter      = (*Store)(nil)
	_ port.NewsReader      = (*Store)(nil)
	_ port.MacroRepository = (*Store)(nil)
	_ port.BondRepository  = (*Store)(nil)
)

// UpsertQuotes implements port.QuoteWriter. An older quote never replaces a
// newer one, so a delayed source cannot overwrite a fresher price.
func (s *Store) UpsertQuotes(ctx context.Context, quotes []domain.Quote) error {
	if len(quotes) == 0 {
		return nil
	}
	n := len(quotes)
	market, symbol, source := make([]string, n), make([]string, n), make([]string, n)
	price, change := make([]float64, n), make([]float64, n)
	asOf := make([]time.Time, n)
	for i, q := range quotes {
		market[i], symbol[i], source[i] = string(q.Asset.Market), string(q.Asset.Symbol), q.Source
		price[i], change[i], asOf[i] = q.Price, q.ChangePct, q.AsOf
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO quotes_latest (asset_id, price, change_pct, as_of, source)
		SELECT DISTINCT ON (a.id) a.id, x.price, x.change_pct, x.as_of, x.source
		FROM unnest($1::text[], $2::text[], $3::float8[], $4::float8[], $5::timestamptz[], $6::text[])
			AS x(market, symbol, price, change_pct, as_of, source)
		JOIN assets a ON a.market = x.market AND a.symbol = x.symbol
		ORDER BY a.id, x.as_of DESC
		ON CONFLICT (asset_id) DO UPDATE SET
			price = EXCLUDED.price, change_pct = EXCLUDED.change_pct, as_of = EXCLUDED.as_of,
			source = EXCLUDED.source, fetched_at = now()
		WHERE quotes_latest.as_of <= EXCLUDED.as_of`,
		market, symbol, price, change, asOf, source)
	if err != nil {
		return fmt.Errorf("upsert quotes: %w", err)
	}
	return nil
}

// LatestQuote implements port.QuoteReader.
func (s *Store) LatestQuote(ctx context.Context, asset domain.AssetKey) (domain.Quote, error) {
	q := domain.Quote{Asset: asset}
	err := s.pool.QueryRow(ctx, `
		SELECT q.price, q.change_pct, q.as_of, q.source
		FROM quotes_latest q JOIN assets a ON a.id = q.asset_id
		WHERE a.market = $1 AND a.symbol = $2`, asset.Market, asset.Symbol).
		Scan(&q.Price, &q.ChangePct, &q.AsOf, &q.Source)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Quote{}, domain.ErrNotFound
	}
	return q, err
}

// UpsertNews implements port.NewsWriter: items are deduplicated by URL and
// linked to every tracked asset they mention; unknown assets are ignored.
func (s *Store) UpsertNews(ctx context.Context, items []domain.NewsItem) error {
	if len(items) == 0 {
		return nil
	}
	var url, title, summary, lang, source []string
	var published []time.Time
	var linkURL, linkMarket, linkSymbol []string
	for _, it := range items {
		url = append(url, it.URL)
		title = append(title, it.Title)
		summary = append(summary, it.Summary)
		lang = append(lang, it.Lang)
		source = append(source, it.Source)
		published = append(published, it.PublishedAt)
		for _, a := range it.Assets {
			linkURL = append(linkURL, it.URL)
			linkMarket = append(linkMarket, string(a.Market))
			linkSymbol = append(linkSymbol, string(a.Symbol))
		}
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO news_items (url, title, summary, lang, source, published_at)
			SELECT DISTINCT ON (x.url) x.* FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::text[], $6::timestamptz[])
				AS x(url, title, summary, lang, source, published_at)
			ON CONFLICT (url) DO UPDATE SET title = EXCLUDED.title, summary = EXCLUDED.summary`,
			url, title, summary, lang, source, published); err != nil {
			return fmt.Errorf("upsert news: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO news_assets (news_id, asset_id)
			SELECT DISTINCT n.id, a.id
			FROM unnest($1::text[], $2::text[], $3::text[]) AS x(url, market, symbol)
			JOIN news_items n ON n.url = x.url
			JOIN assets a ON a.market = x.market AND a.symbol = x.symbol
			ON CONFLICT DO NOTHING`,
			linkURL, linkMarket, linkSymbol); err != nil {
			return fmt.Errorf("link news: %w", err)
		}
		return nil
	})
}

// NewsFor implements port.NewsReader.
func (s *Store) NewsFor(ctx context.Context, asset domain.AssetKey, since time.Time, limit int) ([]domain.NewsItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.url, n.title, n.summary, n.lang, n.source, n.published_at
		FROM news_items n
		JOIN news_assets na ON na.news_id = n.id
		JOIN assets a ON a.id = na.asset_id
		WHERE a.market = $1 AND a.symbol = $2 AND n.published_at >= $3
		ORDER BY n.published_at DESC
		LIMIT $4`, asset.Market, asset.Symbol, since, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.NewsItem, error) {
		it := domain.NewsItem{Assets: []domain.AssetKey{asset}}
		err := r.Scan(&it.URL, &it.Title, &it.Summary, &it.Lang, &it.Source, &it.PublishedAt)
		return it, err
	})
}

// UpsertMacro implements port.MacroRepository.
func (s *Store) UpsertMacro(ctx context.Context, points []domain.MacroPoint) error {
	if len(points) == 0 {
		return nil
	}
	n := len(points)
	code, source := make([]string, n), make([]string, n)
	date := make([]time.Time, n)
	value := make([]float64, n)
	for i, p := range points {
		code[i], source[i], date[i], value[i] = p.Series, p.Source, p.Date, p.Value
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO macro_series (code, date, value, source)
		SELECT DISTINCT ON (x.code, x.date) x.* FROM unnest($1::text[], $2::date[], $3::float8[], $4::text[]) AS x(code, date, value, source)
		ON CONFLICT (code, date) DO UPDATE SET value = EXCLUDED.value, source = EXCLUDED.source, fetched_at = now()`,
		code, date, value, source)
	if err != nil {
		return fmt.Errorf("upsert macro: %w", err)
	}
	return nil
}

// MacroSince implements port.MacroRepository.
func (s *Store) MacroSince(ctx context.Context, series string, since time.Time) ([]domain.MacroPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT date, value, source FROM macro_series WHERE code = $1 AND date >= $2::date ORDER BY date`, series, since)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.MacroPoint, error) {
		p := domain.MacroPoint{Series: series}
		err := r.Scan(&p.Date, &p.Value, &p.Source)
		return p, err
	})
}

// LatestMacroDate implements port.MacroRepository.
func (s *Store) LatestMacroDate(ctx context.Context, series string) (time.Time, bool, error) {
	var d *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT max(date) FROM macro_series WHERE code = $1`, series).Scan(&d); err != nil {
		return time.Time{}, false, err
	}
	if d == nil {
		return time.Time{}, false, nil
	}
	return *d, true, nil
}

// UpsertBondQuotes implements port.BondRepository. The bonds themselves must
// already exist as assets.
func (s *Store) UpsertBondQuotes(ctx context.Context, quotes []domain.BondQuote) error {
	if len(quotes) == 0 {
		return nil
	}
	n := len(quotes)
	market, symbol, source := make([]string, n), make([]string, n), make([]string, n)
	date, maturity := make([]time.Time, n), make([]time.Time, n)
	br, sr, bp, sp := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	for i, q := range quotes {
		market[i], symbol[i], source[i] = string(q.Asset.Market), string(q.Asset.Symbol), q.Source
		date[i], maturity[i] = q.Date, q.Maturity
		br[i], sr[i], bp[i], sp[i] = q.BuyRate, q.SellRate, q.BuyPrice, q.SellPrice
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO bond_quotes (asset_id, date, maturity, buy_rate, sell_rate, buy_price, sell_price, source)
		SELECT DISTINCT ON (a.id, x.date) a.id, x.date, x.maturity, x.br, x.sr, x.bp, x.sp, x.source
		FROM unnest($1::text[], $2::text[], $3::date[], $4::date[], $5::float8[], $6::float8[], $7::float8[], $8::float8[], $9::text[])
			AS x(market, symbol, date, maturity, br, sr, bp, sp, source)
		JOIN assets a ON a.market = x.market AND a.symbol = x.symbol
		ON CONFLICT (asset_id, date) DO UPDATE SET
			buy_rate = EXCLUDED.buy_rate, sell_rate = EXCLUDED.sell_rate, buy_price = EXCLUDED.buy_price,
			sell_price = EXCLUDED.sell_price, source = EXCLUDED.source, fetched_at = now()`,
		market, symbol, date, maturity, br, sr, bp, sp, source)
	if err != nil {
		return fmt.Errorf("upsert bond quotes: %w", err)
	}
	return nil
}

// LatestBondDate implements port.BondRepository.
func (s *Store) LatestBondDate(ctx context.Context) (time.Time, bool, error) {
	var d *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT max(date) FROM bond_quotes`).Scan(&d); err != nil {
		return time.Time{}, false, err
	}
	if d == nil {
		return time.Time{}, false, nil
	}
	return *d, true, nil
}
