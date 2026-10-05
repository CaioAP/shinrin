package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func TestAccounts(t *testing.T) {
	st, _ := testStore(t)
	ctx := context.Background()
	vale := domain.AssetKey{Market: domain.MarketB3, Symbol: "VALE3"}
	if err := st.UpsertAssets(ctx, []domain.Asset{{Key: petr, Class: domain.ClassStock, Active: true}, {Key: vale, Class: domain.ClassStock, Active: true}}); err != nil {
		t.Fatal(err)
	}

	u, err := st.CreateUser(ctx, "caio@example.com", "hash")
	if err != nil || u.ID == 0 {
		t.Fatalf("CreateUser = %+v, %v", u, err)
	}
	if _, err := st.CreateUser(ctx, "caio@example.com", "hash"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate email err = %v", err)
	}
	if _, h, err := st.UserByEmail(ctx, "caio@example.com"); err != nil || h != "hash" {
		t.Errorf("UserByEmail hash %q, %v", h, err)
	}
	at := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	if err := st.SaveRiskProfile(ctx, u.ID, domain.ProfileModerate, domain.SuitabilityAnswers{"horizon": "3_5y"}, at); err != nil {
		t.Fatal(err)
	}
	got, err := st.UserByID(ctx, u.ID)
	if err != nil || got.Profile != domain.ProfileModerate || got.ProfileAnswers["horizon"] != "3_5y" || !got.ProfileAt.Equal(at) {
		t.Errorf("UserByID = %+v, %v", got, err)
	}

	sess := domain.Session{TokenHash: []byte{1, 2, 3}, UserID: u.ID, CreatedAt: at, ExpiresAt: at.Add(time.Hour)}
	if err := st.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if s, err := st.SessionByTokenHash(ctx, sess.TokenHash); err != nil || s.UserID != u.ID || !s.ExpiresAt.Equal(sess.ExpiresAt) {
		t.Errorf("SessionByTokenHash = %+v, %v", s, err)
	}

	other, _ := st.CreateUser(ctx, "other@example.com", "hash")
	w, err := st.CreateWatchlist(ctx, u.ID, "Income")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateWatchlist(ctx, u.ID, "Income"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate name err = %v", err)
	}
	if _, err := st.CreateWatchlist(ctx, other.ID, "Income"); err != nil {
		t.Errorf("another user may reuse the name: %v", err)
	}
	for _, k := range []domain.AssetKey{vale, petr, petr} {
		if err := st.AddWatchlistItem(ctx, u.ID, w.ID, k); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.AddWatchlistItem(ctx, other.ID, w.ID, petr); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other user wrote the list: %v", err)
	}
	if err := st.AddWatchlistItem(ctx, u.ID, w.ID, domain.AssetKey{Market: domain.MarketB3, Symbol: "XXXX3"}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown asset err = %v", err)
	}
	got2, err := st.GetWatchlist(ctx, u.ID, w.ID)
	if err != nil || len(got2.Assets) != 2 || got2.Assets[0] != vale {
		t.Errorf("GetWatchlist = %+v, %v", got2, err)
	}
	if _, err := st.GetWatchlist(ctx, other.ID, w.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other user read the list: %v", err)
	}
	if err := st.RemoveWatchlistItem(ctx, u.ID, w.ID, vale); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameWatchlist(ctx, u.ID, w.ID, "Dividends"); err != nil {
		t.Fatal(err)
	}
	if ls, err := st.ListWatchlists(ctx, u.ID); err != nil || len(ls) != 1 || ls[0].Name != "Dividends" || len(ls[0].Assets) != 1 {
		t.Errorf("ListWatchlists = %+v, %v", ls, err)
	}

	// Deleting the user removes their sessions and lists.
	if err := st.DeleteUser(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SessionByTokenHash(ctx, sess.TokenHash); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("session survived: %v", err)
	}
	if ls, _ := st.ListWatchlists(ctx, u.ID); len(ls) != 0 {
		t.Errorf("lists survived: %+v", ls)
	}
}
