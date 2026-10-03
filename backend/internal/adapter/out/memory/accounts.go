package memory

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// AccountStore keeps users, sessions, watchlists and LLM credentials in
// memory.
type AccountStore struct {
	mu        sync.RWMutex
	users     map[domain.UserID]domain.User
	hashes    map[domain.UserID]string
	sessions  map[string]domain.Session
	lists     map[domain.WatchlistID]domain.Watchlist
	creds     map[domain.UserID]port.StoredCredential
	nextUser  domain.UserID
	nextList  domain.WatchlistID
	createdAt func() time.Time
}

var (
	_ port.UserRepository       = (*AccountStore)(nil)
	_ port.SessionRepository    = (*AccountStore)(nil)
	_ port.WatchlistRepository  = (*AccountStore)(nil)
	_ port.CredentialRepository = (*AccountStore)(nil)
)

// NewAccountStore returns an empty store.
func NewAccountStore() *AccountStore {
	return &AccountStore{
		users: map[domain.UserID]domain.User{}, hashes: map[domain.UserID]string{},
		sessions: map[string]domain.Session{}, lists: map[domain.WatchlistID]domain.Watchlist{},
		creds:     map[domain.UserID]port.StoredCredential{},
		createdAt: time.Now,
	}
}

// CreateUser implements port.UserRepository.
func (s *AccountStore) CreateUser(_ context.Context, email, hash string) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.Email == email {
			return domain.User{}, domain.ErrConflict
		}
	}
	s.nextUser++
	u := domain.User{ID: s.nextUser, Email: email, CreatedAt: s.createdAt()}
	s.users[u.ID], s.hashes[u.ID] = u, hash
	return u, nil
}

// UserByEmail implements port.UserRepository.
func (s *AccountStore) UserByEmail(_ context.Context, email string) (domain.User, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Email == email {
			return u, s.hashes[u.ID], nil
		}
	}
	return domain.User{}, "", domain.ErrNotFound
}

// UserByID implements port.UserRepository.
func (s *AccountStore) UserByID(_ context.Context, id domain.UserID) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

// PasswordHash implements port.UserRepository.
func (s *AccountStore) PasswordHash(_ context.Context, id domain.UserID) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.hashes[id]
	if !ok {
		return "", domain.ErrNotFound
	}
	return h, nil
}

// SaveRiskProfile implements port.UserRepository.
func (s *AccountStore) SaveRiskProfile(_ context.Context, id domain.UserID, p domain.RiskProfile, a domain.SuitabilityAnswers, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return domain.ErrNotFound
	}
	u.Profile, u.ProfileAnswers, u.ProfileAt = p, maps.Clone(a), at
	s.users[id] = u
	return nil
}

// DeleteUser implements port.UserRepository.
func (s *AccountStore) DeleteUser(_ context.Context, id domain.UserID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, id)
	delete(s.hashes, id)
	delete(s.creds, id)
	for k, v := range s.sessions {
		if v.UserID == id {
			delete(s.sessions, k)
		}
	}
	for k, v := range s.lists {
		if v.UserID == id {
			delete(s.lists, k)
		}
	}
	return nil
}

// CreateSession implements port.SessionRepository.
func (s *AccountStore) CreateSession(_ context.Context, sess domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[string(sess.TokenHash)] = sess
	return nil
}

// SessionByTokenHash implements port.SessionRepository.
func (s *AccountStore) SessionByTokenHash(_ context.Context, hash []byte) (domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[string(hash)]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}
	return sess, nil
}

// DeleteSession implements port.SessionRepository.
func (s *AccountStore) DeleteSession(_ context.Context, hash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, string(hash))
	return nil
}

// ListWatchlists implements port.WatchlistRepository.
func (s *AccountStore) ListWatchlists(_ context.Context, user domain.UserID) ([]domain.Watchlist, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Watchlist{}
	for _, w := range s.lists {
		if w.UserID == user {
			w.Assets = slices.Clone(w.Assets)
			out = append(out, w)
		}
	}
	slices.SortFunc(out, func(a, b domain.Watchlist) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

// GetWatchlist implements port.WatchlistRepository.
func (s *AccountStore) GetWatchlist(_ context.Context, user domain.UserID, id domain.WatchlistID) (domain.Watchlist, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.lists[id]
	if !ok || w.UserID != user {
		return domain.Watchlist{}, domain.ErrNotFound
	}
	w.Assets = slices.Clone(w.Assets)
	return w, nil
}

// CreateWatchlist implements port.WatchlistRepository.
func (s *AccountStore) CreateWatchlist(_ context.Context, user domain.UserID, name string) (domain.Watchlist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nameTaken(user, 0, name) {
		return domain.Watchlist{}, domain.ErrConflict
	}
	s.nextList++
	w := domain.Watchlist{ID: s.nextList, UserID: user, Name: name, Assets: []domain.AssetKey{}}
	s.lists[w.ID] = w
	return w, nil
}

// RenameWatchlist implements port.WatchlistRepository.
func (s *AccountStore) RenameWatchlist(_ context.Context, user domain.UserID, id domain.WatchlistID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.lists[id]
	if !ok || w.UserID != user {
		return domain.ErrNotFound
	}
	if s.nameTaken(user, id, name) {
		return domain.ErrConflict
	}
	w.Name = name
	s.lists[id] = w
	return nil
}

// DeleteWatchlist implements port.WatchlistRepository.
func (s *AccountStore) DeleteWatchlist(_ context.Context, user domain.UserID, id domain.WatchlistID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.lists[id]; !ok || w.UserID != user {
		return domain.ErrNotFound
	}
	delete(s.lists, id)
	return nil
}

// AddWatchlistItem implements port.WatchlistRepository.
func (s *AccountStore) AddWatchlistItem(_ context.Context, user domain.UserID, id domain.WatchlistID, a domain.AssetKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.lists[id]
	if !ok || w.UserID != user {
		return domain.ErrNotFound
	}
	if !slices.Contains(w.Assets, a) {
		w.Assets = append(w.Assets, a)
	}
	s.lists[id] = w
	return nil
}

// RemoveWatchlistItem implements port.WatchlistRepository.
func (s *AccountStore) RemoveWatchlistItem(_ context.Context, user domain.UserID, id domain.WatchlistID, a domain.AssetKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.lists[id]
	if !ok || w.UserID != user {
		return domain.ErrNotFound
	}
	w.Assets = slices.DeleteFunc(w.Assets, func(k domain.AssetKey) bool { return k == a })
	s.lists[id] = w
	return nil
}

func (s *AccountStore) nameTaken(user domain.UserID, except domain.WatchlistID, name string) bool {
	for _, w := range s.lists {
		if w.UserID == user && w.ID != except && w.Name == name {
			return true
		}
	}
	return false
}

// Credential implements port.CredentialRepository.
func (s *AccountStore) Credential(_ context.Context, user domain.UserID) (port.StoredCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.creds[user]
	if !ok {
		return port.StoredCredential{}, domain.ErrNotFound
	}
	c.SealedKey = slices.Clone(c.SealedKey)
	return c, nil
}

// SaveCredential implements port.CredentialRepository.
func (s *AccountStore) SaveCredential(_ context.Context, user domain.UserID, c port.StoredCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[user]; !ok {
		return domain.ErrNotFound
	}
	c.SealedKey = slices.Clone(c.SealedKey)
	s.creds[user] = c
	return nil
}

// DeleteCredential implements port.CredentialRepository.
func (s *AccountStore) DeleteCredential(_ context.Context, user domain.UserID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.creds, user)
	return nil
}
