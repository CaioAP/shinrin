package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

var (
	_ port.UserRepository      = (*Store)(nil)
	_ port.SessionRepository   = (*Store)(nil)
	_ port.WatchlistRepository = (*Store)(nil)
)

// isUniqueViolation reports a duplicate key, which callers map to
// domain.ErrConflict.
func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

const userColumns = `id, email, created_at, risk_profile, risk_answers, risk_updated_at`

func scanUser(row pgx.Row, extra ...any) (domain.User, error) {
	var (
		u       domain.User
		profile *string
		answers []byte
		at      *time.Time
	)
	err := row.Scan(append([]any{&u.ID, &u.Email, &u.CreatedAt, &profile, &answers, &at}, extra...)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	if profile != nil {
		u.Profile = domain.RiskProfile(*profile)
	}
	if at != nil {
		u.ProfileAt = *at
	}
	if len(answers) > 0 {
		if err := json.Unmarshal(answers, &u.ProfileAnswers); err != nil {
			return domain.User{}, err
		}
	}
	return u, nil
}

// CreateUser implements port.UserRepository.
func (s *Store) CreateUser(ctx context.Context, email, hash string) (domain.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING `+userColumns, email, hash))
	if isUniqueViolation(err) {
		return domain.User{}, domain.ErrConflict
	}
	return u, err
}

// UserByEmail implements port.UserRepository.
func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, string, error) {
	var hash string
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+`, password_hash FROM users WHERE email = $1`, email), &hash)
	return u, hash, err
}

// UserByID implements port.UserRepository.
func (s *Store) UserByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// PasswordHash implements port.UserRepository.
func (s *Store) PasswordHash(ctx context.Context, id domain.UserID) (string, error) {
	var h string
	err := s.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, id).Scan(&h)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return h, err
}

// SaveRiskProfile implements port.UserRepository.
func (s *Store) SaveRiskProfile(ctx context.Context, id domain.UserID, p domain.RiskProfile, a domain.SuitabilityAnswers, at time.Time) error {
	answers, err := json.Marshal(a)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE users SET risk_profile = $2, risk_answers = $3, risk_updated_at = $4 WHERE id = $1`, id, p, answers, at)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

// DeleteUser implements port.UserRepository. Foreign keys cascade to
// sessions, watchlists and reports.
func (s *Store) DeleteUser(ctx context.Context, id domain.UserID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// CreateSession implements port.SessionRepository. Expired sessions of the
// same user are swept on the way, so the table does not grow forever.
func (s *Store) CreateSession(ctx context.Context, sess domain.Session) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND expires_at <= $2`, sess.UserID, sess.CreatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES ($1, $2, $3, $4)`,
			sess.TokenHash, sess.UserID, sess.CreatedAt, sess.ExpiresAt)
		return err
	})
}

// SessionByTokenHash implements port.SessionRepository.
func (s *Store) SessionByTokenHash(ctx context.Context, hash []byte) (domain.Session, error) {
	sess := domain.Session{TokenHash: hash}
	err := s.pool.QueryRow(ctx, `SELECT user_id, created_at, expires_at FROM sessions WHERE token_hash = $1`, hash).
		Scan(&sess.UserID, &sess.CreatedAt, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrNotFound
	}
	return sess, err
}

// DeleteSession implements port.SessionRepository.
func (s *Store) DeleteSession(ctx context.Context, hash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash)
	return err
}

// ListWatchlists implements port.WatchlistRepository.
func (s *Store) ListWatchlists(ctx context.Context, user domain.UserID) ([]domain.Watchlist, error) {
	return s.watchlists(ctx, `w.user_id = $1`, user)
}

// GetWatchlist implements port.WatchlistRepository.
func (s *Store) GetWatchlist(ctx context.Context, user domain.UserID, id domain.WatchlistID) (domain.Watchlist, error) {
	l, err := s.watchlists(ctx, `w.user_id = $1 AND w.id = $2`, user, id)
	if err != nil {
		return domain.Watchlist{}, err
	}
	if len(l) == 0 {
		return domain.Watchlist{}, domain.ErrNotFound
	}
	return l[0], nil
}

func (s *Store) watchlists(ctx context.Context, where string, args ...any) ([]domain.Watchlist, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT w.id, w.user_id, w.name, a.market, a.symbol
		FROM watchlists w
		LEFT JOIN watchlist_items i ON i.watchlist_id = w.id
		LEFT JOIN assets a ON a.id = i.asset_id
		WHERE `+where+`
		ORDER BY w.id, i.added_at, a.market, a.symbol`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Watchlist{}
	for rows.Next() {
		var (
			w              domain.Watchlist
			market, symbol *string
		)
		if err := rows.Scan(&w.ID, &w.UserID, &w.Name, &market, &symbol); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].ID != w.ID {
			w.Assets = []domain.AssetKey{}
			out = append(out, w)
		}
		if market != nil {
			last := &out[len(out)-1]
			last.Assets = append(last.Assets, domain.AssetKey{Market: domain.Market(*market), Symbol: domain.Symbol(*symbol)})
		}
	}
	return out, rows.Err()
}

// CreateWatchlist implements port.WatchlistRepository.
func (s *Store) CreateWatchlist(ctx context.Context, user domain.UserID, name string) (domain.Watchlist, error) {
	w := domain.Watchlist{UserID: user, Name: name, Assets: []domain.AssetKey{}}
	err := s.pool.QueryRow(ctx, `INSERT INTO watchlists (user_id, name) VALUES ($1, $2) RETURNING id`, user, name).Scan(&w.ID)
	if isUniqueViolation(err) {
		return domain.Watchlist{}, domain.ErrConflict
	}
	return w, err
}

// RenameWatchlist implements port.WatchlistRepository.
func (s *Store) RenameWatchlist(ctx context.Context, user domain.UserID, id domain.WatchlistID, name string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE watchlists SET name = $3 WHERE id = $2 AND user_id = $1`, user, id, name)
	switch {
	case isUniqueViolation(err):
		return domain.ErrConflict
	case err != nil:
		return err
	case tag.RowsAffected() == 0:
		return domain.ErrNotFound
	}
	return nil
}

// DeleteWatchlist implements port.WatchlistRepository.
func (s *Store) DeleteWatchlist(ctx context.Context, user domain.UserID, id domain.WatchlistID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM watchlists WHERE id = $2 AND user_id = $1`, user, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

// AddWatchlistItem implements port.WatchlistRepository. The ownership check
// is in the same statement, so no other user's list can be written to.
func (s *Store) AddWatchlistItem(ctx context.Context, user domain.UserID, id domain.WatchlistID, a domain.AssetKey) error {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO watchlist_items (watchlist_id, asset_id)
		SELECT w.id, a.id FROM watchlists w, assets a
		WHERE w.id = $2 AND w.user_id = $1 AND a.market = $3 AND a.symbol = $4
		ON CONFLICT DO NOTHING`, user, id, a.Market, a.Symbol)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Either already listed (fine) or the list or asset is not there.
		if _, err := s.GetWatchlist(ctx, user, id); err != nil {
			return err
		}
		if _, err := s.GetAsset(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

// RemoveWatchlistItem implements port.WatchlistRepository.
func (s *Store) RemoveWatchlistItem(ctx context.Context, user domain.UserID, id domain.WatchlistID, a domain.AssetKey) error {
	if _, err := s.GetWatchlist(ctx, user, id); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM watchlist_items i USING assets a
		WHERE i.watchlist_id = $1 AND i.asset_id = a.id AND a.market = $2 AND a.symbol = $3`, id, a.Market, a.Symbol)
	return err
}
