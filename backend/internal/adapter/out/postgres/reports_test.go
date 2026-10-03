package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

func TestUserReportsAndCredentials(t *testing.T) {
	st, _ := testStore(t)
	ctx := context.Background()
	if err := st.UpsertAssets(ctx, []domain.Asset{{Key: petr, Class: domain.ClassStock, Active: true}}); err != nil {
		t.Fatal(err)
	}
	u, _ := st.CreateUser(ctx, "a@example.com", "h")
	other, _ := st.CreateUser(ctx, "b@example.com", "h")

	at := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	save := func(user domain.UserID, created time.Time) int64 {
		id, err := st.SaveReport(ctx, domain.Report{
			UserID: user, Kind: domain.ReportKindAsset, Asset: petr, Profile: domain.ProfileModerate, AsOf: d(2026, 10, 2),
			Snapshot: domain.ReportSnapshot{Facts: map[string]float64{"pe": 4.1}, Labels: map[string]string{"symbol": "PETR4"}},
			Output:   domain.ReportOutput{Summary: "s", BullCase: []string{"b"}, AllocationMinPct: 1, AllocationMaxPct: 5, Confidence: "medium"},
			Provider: "anthropic", Model: "m", TokensIn: 10, TokensOut: 5, CreatedAt: created,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	save(u.ID, at.AddDate(0, -1, 0))
	last := save(u.ID, at)
	save(other.ID, at)
	save(0, at) // a CLI report has no owner

	list, err := st.ReportsFor(ctx, u.ID, petr, 10)
	if err != nil || len(list) != 2 || list[0].ID != last {
		t.Fatalf("ReportsFor = %+v, %v", list, err)
	}
	r := list[0]
	if r.UserID != u.ID || r.Output.AllocationMaxPct != 5 || r.Output.BullCase[0] != "b" || r.Snapshot.Facts["pe"] != 4.1 || !r.CreatedAt.Equal(at) {
		t.Errorf("report = %+v", r)
	}
	if _, err := st.Report(ctx, other.ID, last); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other user's report err = %v", err)
	}
	if n, err := st.CountReportsSince(ctx, u.ID, domain.MonthStart(at)); err != nil || n != 1 {
		t.Errorf("CountReportsSince = %d, %v", n, err)
	}

	if _, err := st.Credential(ctx, u.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("no credential err = %v", err)
	}
	c := port.StoredCredential{Settings: domain.LLMSettings{Provider: "anthropic", KeyHint: "…abcd", MonthlyCap: 30, UpdatedAt: at}, SealedKey: []byte{1, 2, 3}}
	if err := st.SaveCredential(ctx, u.ID, c); err != nil {
		t.Fatal(err)
	}
	c.Settings.Model, c.Settings.MonthlyCap = "claude-x", 5
	if err := st.SaveCredential(ctx, u.ID, c); err != nil {
		t.Fatal(err)
	}
	got, err := st.Credential(ctx, u.ID)
	if err != nil || got.Settings.Model != "claude-x" || got.Settings.MonthlyCap != 5 || len(got.SealedKey) != 3 {
		t.Fatalf("Credential = %+v, %v", got, err)
	}
	if err := st.SaveCredential(ctx, 999999, c); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("credential for unknown user err = %v", err)
	}

	// Deleting the account deletes its key and reports.
	if err := st.DeleteUser(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Credential(ctx, u.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("credential after account deletion err = %v", err)
	}
	if n, _ := st.CountReportsSince(ctx, u.ID, time.Time{}); n != 0 {
		t.Errorf("%d reports left after account deletion", n)
	}
	if err := st.DeleteCredential(ctx, other.ID); err != nil {
		t.Error(err)
	}
}
