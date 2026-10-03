package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

var (
	_ port.ReportReader         = (*Store)(nil)
	_ port.CredentialRepository = (*Store)(nil)
)

func nullUser(id domain.UserID) *int64 {
	if id == 0 {
		return nil
	}
	v := int64(id)
	return &v
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

const reportColumns = `r.id, r.user_id, a.market, a.symbol, r.kind, r.profile, r.as_of, r.input_snapshot, r.output, r.omitted,
	r.provider, r.model, r.tokens_in, r.tokens_out, r.created_at`

// ReportsFor implements port.ReportReader.
func (s *Store) ReportsFor(ctx context.Context, user domain.UserID, asset domain.AssetKey, limit int) ([]domain.Report, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+reportColumns+`
		FROM ai_reports r JOIN assets a ON a.id = r.asset_id
		WHERE r.user_id = $1 AND a.market = $2 AND a.symbol = $3
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $4`, int64(user), asset.Market, asset.Symbol, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanReport)
}

// Report implements port.ReportReader.
func (s *Store) Report(ctx context.Context, user domain.UserID, id int64) (domain.Report, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+reportColumns+`
		FROM ai_reports r JOIN assets a ON a.id = r.asset_id
		WHERE r.user_id = $1 AND r.id = $2`, int64(user), id)
	if err != nil {
		return domain.Report{}, err
	}
	r, err := pgx.CollectExactlyOneRow(rows, scanReport)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Report{}, domain.ErrNotFound
	}
	return r, err
}

// CountReportsSince implements port.ReportReader.
func (s *Store) CountReportsSince(ctx context.Context, user domain.UserID, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM ai_reports WHERE user_id = $1 AND created_at >= $2`, int64(user), since).Scan(&n)
	return n, err
}

func scanReport(row pgx.CollectableRow) (domain.Report, error) {
	var (
		r         domain.Report
		user      *int64
		snapJSON  []byte
		outJSON   []byte
		snap      snapshotJSON
		out       outputJSON
		createdAt time.Time
	)
	if err := row.Scan(&r.ID, &user, &r.Asset.Market, &r.Asset.Symbol, &r.Kind, &r.Profile, &r.AsOf, &snapJSON, &outJSON, &r.Omitted,
		&r.Provider, &r.Model, &r.TokensIn, &r.TokensOut, &createdAt); err != nil {
		return domain.Report{}, err
	}
	if user != nil {
		r.UserID = domain.UserID(*user)
	}
	r.CreatedAt = createdAt
	if err := json.Unmarshal(snapJSON, &snap); err != nil {
		return domain.Report{}, fmt.Errorf("report %d snapshot: %w", r.ID, err)
	}
	if err := json.Unmarshal(outJSON, &out); err != nil {
		return domain.Report{}, fmt.Errorf("report %d output: %w", r.ID, err)
	}
	r.Snapshot = domain.ReportSnapshot{Facts: snap.Facts, Labels: snap.Labels}
	for _, h := range snap.Headlines {
		r.Snapshot.Headlines = append(r.Snapshot.Headlines, domain.Headline{ID: h.ID, Title: h.Title, Source: h.Source, PublishedAt: h.PublishedAt})
	}
	r.Output = domain.ReportOutput{Summary: out.Summary, BullCase: out.BullCase, BearCase: out.BearCase, ValuationView: out.ValuationView,
		TimingView: out.TimingView, FitForProfile: out.FitForProfile, AllocationMinPct: out.Allocation.Min, AllocationMaxPct: out.Allocation.Max,
		KeyRisks: out.KeyRisks, Confidence: out.Confidence, CitedData: out.CitedData}
	return r, nil
}

// Credential implements port.CredentialRepository.
func (s *Store) Credential(ctx context.Context, user domain.UserID) (port.StoredCredential, error) {
	var c port.StoredCredential
	st := &c.Settings
	err := s.pool.QueryRow(ctx, `
		SELECT provider, model, base_url, key_hint, sealed_key, monthly_cap, updated_at
		FROM llm_credentials WHERE user_id = $1`, int64(user)).
		Scan(&st.Provider, &st.Model, &st.BaseURL, &st.KeyHint, &c.SealedKey, &st.MonthlyCap, &st.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.StoredCredential{}, domain.ErrNotFound
	}
	return c, err
}

// SaveCredential implements port.CredentialRepository.
func (s *Store) SaveCredential(ctx context.Context, user domain.UserID, c port.StoredCredential) error {
	st := c.Settings
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO llm_credentials (user_id, provider, model, base_url, key_hint, sealed_key, monthly_cap, updated_at)
		SELECT id, $2, $3, $4, $5, $6, $7, $8 FROM users WHERE id = $1
		ON CONFLICT (user_id) DO UPDATE SET provider = excluded.provider, model = excluded.model, base_url = excluded.base_url,
			key_hint = excluded.key_hint, sealed_key = excluded.sealed_key, monthly_cap = excluded.monthly_cap, updated_at = excluded.updated_at`,
		int64(user), st.Provider, st.Model, st.BaseURL, st.KeyHint, c.SealedKey, st.MonthlyCap, st.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save credential: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteCredential implements port.CredentialRepository.
func (s *Store) DeleteCredential(ctx context.Context, user domain.UserID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM llm_credentials WHERE user_id = $1`, int64(user))
	return err
}
