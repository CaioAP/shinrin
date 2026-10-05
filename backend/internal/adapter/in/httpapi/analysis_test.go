package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/httpapi"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

type fakeAnalysis struct {
	gotProfile domain.RiskProfile
	gotFilter  port.RankFilter
}

var petr = domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}

func (f *fakeAnalysis) Analyze(_ context.Context, k domain.AssetKey, p domain.RiskProfile) (domain.Analysis, error) {
	f.gotProfile = p
	if k != petr {
		return domain.Analysis{}, domain.ErrNotFound
	}
	return domain.Analysis{
		Asset: domain.Asset{Key: petr, Class: domain.ClassStock, Name: "Petrobras"}, AsOf: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		Profile: p, Price: 38.5, Composite: 72.4, Coverage: 0.85,
		Scorecard: domain.Scorecard{Factors: []domain.FactorScore{{Factor: domain.FactorValuation, Value: 90, PeerGroup: "B3 stock",
			Inputs: []domain.ScoreInput{{Metric: "pe", Value: 4, Points: 95, Weight: 1, Peers: 40}}}}},
		FairValues: []domain.FairValue{{Method: "bazin", Low: 60, High: 80, Assumptions: map[string]float64{"min_yield": 0.06}}},
		Valuation:  domain.ValuationCheap,
		Timing:     domain.Timing{View: domain.TimingWait, Signals: []domain.Signal{{Code: "rsi_overbought", Message: "m", Weight: -1}}},
	}, nil
}

func (f *fakeAnalysis) Rank(_ context.Context, flt port.RankFilter) ([]port.RankedAsset, error) {
	f.gotFilter = flt
	return []port.RankedAsset{{Asset: domain.Asset{Key: petr, Class: domain.ClassStock}, Composite: 80,
		Factors: []domain.FactorScore{{Factor: domain.FactorIncome, Value: 77}}}}, nil
}

func (f *fakeAnalysis) Outlook(_ context.Context, p domain.RiskProfile) (domain.Outlook, error) {
	return domain.Outlook{Profile: p, Bands: []domain.AllocationBand{{Class: "fixed_income", Min: 60, Max: 80, Lean: "high"}}}, nil
}

func analysisServer(f *fakeAnalysis) http.Handler {
	return httpapi.NewRouter(httpapi.Deps{System: fakeSystem{healthy: true}, Catalog: &fakeCatalog{}, Analysis: f,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func decode(t *testing.T, r io.Reader) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestAnalysisEndpoint(t *testing.T) {
	f := &fakeAnalysis{}
	h := analysisServer(f)
	rec := do(t, h, "/api/v1/assets/b3/petr4/analysis?profile=Aggressive")
	if rec.Code != http.StatusOK || f.gotProfile != domain.ProfileAggressive {
		t.Fatalf("status %d, profile %q", rec.Code, f.gotProfile)
	}
	body := decode(t, rec.Body)
	if body["disclaimer"] != domain.Disclaimer || body["asOf"] != "2026-10-02" || body["composite"] != 72.4 {
		t.Errorf("body = %v", body)
	}
	factor := body["factors"].([]any)[0].(map[string]any)
	if factor["factor"] != "valuation" || factor["inputs"].([]any)[0].(map[string]any)["peers"] != 40.0 {
		t.Errorf("factor = %v", factor)
	}
	if body["timing"].(map[string]any)["view"] != "wait" || body["valuation"].(map[string]any)["view"] != "cheap" {
		t.Errorf("views = %v %v", body["timing"], body["valuation"])
	}
	if notes, ok := body["notes"].([]any); !ok || len(notes) != 0 {
		t.Errorf("notes should be an empty list, got %v", body["notes"])
	}

	_ = do(t, h, "/api/v1/assets/B3/PETR4/analysis")
	if f.gotProfile != domain.ProfileModerate {
		t.Errorf("default profile = %q", f.gotProfile)
	}
	for path, want := range map[string]int{
		"/api/v1/assets/B3/NOPE3/analysis":              http.StatusNotFound,
		"/api/v1/assets/B3/PETR4/analysis?profile=yolo": http.StatusBadRequest,
		"/api/v1/rankings?limit=0":                      http.StatusBadRequest,
		"/api/v1/outlook?profile=yolo":                  http.StatusBadRequest,
	} {
		if rec := do(t, h, path); rec.Code != want {
			t.Errorf("%s = %d, want %d", path, rec.Code, want)
		}
	}
}

func TestRankingsAndOutlook(t *testing.T) {
	f := &fakeAnalysis{}
	h := analysisServer(f)
	rec := do(t, h, "/api/v1/rankings?market=US&class=reit&profile=conservative&limit=10")
	want := port.RankFilter{Market: domain.MarketUS, Class: domain.ClassREIT, Profile: domain.ProfileConservative, Limit: 10}
	if rec.Code != http.StatusOK || !reflect.DeepEqual(f.gotFilter, want) {
		t.Fatalf("status %d, filter %+v", rec.Code, f.gotFilter)
	}
	body := decode(t, rec.Body)
	item := body["items"].([]any)[0].(map[string]any)
	if body["disclaimer"] != domain.Disclaimer || item["factors"].(map[string]any)["income"] != 77.0 {
		t.Errorf("ranking = %v", body)
	}
	do(t, h, "/api/v1/rankings")
	if f.gotFilter.Limit != 50 {
		t.Errorf("default limit = %d", f.gotFilter.Limit)
	}

	rec = do(t, h, "/api/v1/outlook?profile=conservative")
	body = decode(t, rec.Body)
	band := body["bands"].([]any)[0].(map[string]any)
	if rec.Code != http.StatusOK || band["lean"] != "high" || body["disclaimer"] != domain.Disclaimer {
		t.Errorf("outlook = %v", body)
	}
}
