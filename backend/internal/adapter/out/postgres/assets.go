package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

const assetColumns = `market, symbol, class, name, sector, isin, cik, cnpj, is_index_member, active`

func scanAsset(row pgx.Row) (domain.Asset, error) {
	var a domain.Asset
	err := row.Scan(&a.Key.Market, &a.Key.Symbol, &a.Class, &a.Name, &a.Sector, &a.ISIN, &a.CIK, &a.CNPJ, &a.IndexMember, &a.Active)
	return a, err
}

// ListAssets implements port.AssetReader.
func (s *Store) ListAssets(ctx context.Context, f port.AssetFilter) ([]domain.Asset, error) {
	var (
		where []string
		args  []any
	)
	if f.Market != "" {
		args = append(args, f.Market)
		where = append(where, fmt.Sprintf("market = $%d", len(args)))
	}
	if f.Class != "" {
		args = append(args, f.Class)
		where = append(where, fmt.Sprintf("class = $%d", len(args)))
	}
	if f.IndexMember != nil {
		args = append(args, *f.IndexMember)
		where = append(where, fmt.Sprintf("is_index_member = $%d", len(args)))
	}
	q := "SELECT " + assetColumns + " FROM assets"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY market, symbol"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Asset, error) { return scanAsset(r) })
}

// GetAsset implements port.AssetReader.
func (s *Store) GetAsset(ctx context.Context, key domain.AssetKey) (domain.Asset, error) {
	a, err := scanAsset(s.pool.QueryRow(ctx,
		"SELECT "+assetColumns+" FROM assets WHERE market = $1 AND symbol = $2", key.Market, key.Symbol))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Asset{}, domain.ErrNotFound
	}
	return a, err
}

// UpsertAssets implements port.AssetWriter.
func (s *Store) UpsertAssets(ctx context.Context, assets []domain.Asset) error {
	if len(assets) == 0 {
		return nil
	}
	n := len(assets)
	market, symbol, class := make([]string, n), make([]string, n), make([]string, n)
	name, sector, isin, cik, cnpj := make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	member, active := make([]bool, n), make([]bool, n)
	for i, a := range assets {
		market[i], symbol[i], class[i] = string(a.Key.Market), string(a.Key.Symbol), string(a.Class)
		name[i], sector[i], isin[i], cik[i], cnpj[i] = a.Name, a.Sector, a.ISIN, a.CIK, a.CNPJ
		member[i], active[i] = a.IndexMember, a.Active
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO assets (`+assetColumns+`)
		SELECT DISTINCT ON (market, symbol) * FROM unnest(
			$1::text[], $2::text[], $3::text[], $4::text[], $5::text[],
			$6::text[], $7::text[], $8::text[], $9::bool[], $10::bool[])
			AS x(`+assetColumns+`)
		ON CONFLICT (market, symbol) DO UPDATE SET
			class = EXCLUDED.class, name = EXCLUDED.name, sector = EXCLUDED.sector,
			isin = EXCLUDED.isin, cik = EXCLUDED.cik, cnpj = EXCLUDED.cnpj,
			is_index_member = EXCLUDED.is_index_member, active = EXCLUDED.active,
			updated_at = now()`,
		market, symbol, class, name, sector, isin, cik, cnpj, member, active)
	if err != nil {
		return fmt.Errorf("upsert assets: %w", err)
	}
	return nil
}
