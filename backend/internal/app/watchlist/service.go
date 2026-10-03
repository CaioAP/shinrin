// Package watchlist implements port.WatchlistService: a user's named lists of
// assets, shown with their scores for the user's profile.
package watchlist

import (
	"context"
	"fmt"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// MaxLists and MaxItems bound what one user can store.
const (
	MaxLists = 20
	MaxItems = 100
)

// Deps are the ports the service needs. Analysis supplies the scores.
type Deps struct {
	Lists    port.WatchlistRepository
	Assets   port.AssetReader
	Analysis port.AnalysisService
}

// Service implements port.WatchlistService.
type Service struct{ d Deps }

var _ port.WatchlistService = (*Service)(nil)

// New builds the service.
func New(d Deps) *Service { return &Service{d: d} }

// List implements port.WatchlistService.
func (s *Service) List(ctx context.Context, user domain.UserID) ([]domain.Watchlist, error) {
	l, err := s.d.Lists.ListWatchlists(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("list watchlists: %w", err)
	}
	return l, nil
}

// Create implements port.WatchlistService.
func (s *Service) Create(ctx context.Context, user domain.UserID, name string) (domain.Watchlist, error) {
	name, err := domain.NormalizeWatchlistName(name)
	if err != nil {
		return domain.Watchlist{}, err
	}
	existing, err := s.d.Lists.ListWatchlists(ctx, user)
	if err != nil {
		return domain.Watchlist{}, fmt.Errorf("list watchlists: %w", err)
	}
	if len(existing) >= MaxLists {
		return domain.Watchlist{}, fmt.Errorf("%w: at most %d watchlists", domain.ErrInvalid, MaxLists)
	}
	w, err := s.d.Lists.CreateWatchlist(ctx, user, name)
	if err != nil {
		return domain.Watchlist{}, fmt.Errorf("create watchlist: %w", err)
	}
	return w, nil
}

// Rename implements port.WatchlistService.
func (s *Service) Rename(ctx context.Context, user domain.UserID, id domain.WatchlistID, name string) error {
	name, err := domain.NormalizeWatchlistName(name)
	if err != nil {
		return err
	}
	if err := s.d.Lists.RenameWatchlist(ctx, user, id, name); err != nil {
		return fmt.Errorf("rename watchlist %d: %w", id, err)
	}
	return nil
}

// Delete implements port.WatchlistService.
func (s *Service) Delete(ctx context.Context, user domain.UserID, id domain.WatchlistID) error {
	if err := s.d.Lists.DeleteWatchlist(ctx, user, id); err != nil {
		return fmt.Errorf("delete watchlist %d: %w", id, err)
	}
	return nil
}

// AddAsset implements port.WatchlistService.
func (s *Service) AddAsset(ctx context.Context, user domain.UserID, id domain.WatchlistID, key domain.AssetKey) error {
	if _, err := s.d.Assets.GetAsset(ctx, key); err != nil {
		return fmt.Errorf("asset %s: %w", key, err)
	}
	w, err := s.d.Lists.GetWatchlist(ctx, user, id)
	if err != nil {
		return fmt.Errorf("watchlist %d: %w", id, err)
	}
	if len(w.Assets) >= MaxItems {
		return fmt.Errorf("%w: at most %d assets per watchlist", domain.ErrInvalid, MaxItems)
	}
	if err := s.d.Lists.AddWatchlistItem(ctx, user, id, key); err != nil {
		return fmt.Errorf("add %s to watchlist %d: %w", key, id, err)
	}
	return nil
}

// RemoveAsset implements port.WatchlistService.
func (s *Service) RemoveAsset(ctx context.Context, user domain.UserID, id domain.WatchlistID, key domain.AssetKey) error {
	if err := s.d.Lists.RemoveWatchlistItem(ctx, user, id, key); err != nil {
		return fmt.Errorf("remove %s from watchlist %d: %w", key, id, err)
	}
	return nil
}

// Entries implements port.WatchlistService.
func (s *Service) Entries(ctx context.Context, user domain.UserID, id domain.WatchlistID, p domain.RiskProfile) (domain.Watchlist, []port.WatchlistEntry, error) {
	w, err := s.d.Lists.GetWatchlist(ctx, user, id)
	if err != nil {
		return domain.Watchlist{}, nil, fmt.Errorf("watchlist %d: %w", id, err)
	}
	if len(w.Assets) == 0 {
		return w, []port.WatchlistEntry{}, nil
	}
	ranked, err := s.d.Analysis.Rank(ctx, port.RankFilter{Profile: p, Assets: w.Assets})
	if err != nil {
		return domain.Watchlist{}, nil, fmt.Errorf("rank watchlist %d: %w", id, err)
	}
	out := make([]port.WatchlistEntry, 0, len(w.Assets))
	seen := make(map[domain.AssetKey]bool, len(ranked))
	for i := range ranked {
		out = append(out, port.WatchlistEntry{Asset: ranked[i].Asset, Ranked: &ranked[i]})
		seen[ranked[i].Asset.Key] = true
	}
	for _, k := range w.Assets {
		if seen[k] {
			continue
		}
		a, err := s.d.Assets.GetAsset(ctx, k)
		if err != nil {
			return domain.Watchlist{}, nil, fmt.Errorf("asset %s: %w", k, err)
		}
		out = append(out, port.WatchlistEntry{Asset: a})
	}
	return w, out, nil
}
