package report_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/memory"
	"github.com/CaioAP/shinrin/backend/internal/app/report"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/scoring"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

var (
	asOf = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	petr = domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"}
)

type fakeAnalysis struct{}

func (fakeAnalysis) Analyze(_ context.Context, k domain.AssetKey, p domain.RiskProfile) (domain.Analysis, error) {
	if k != petr {
		return domain.Analysis{}, domain.ErrNotFound
	}
	return domain.Analysis{
		Asset: domain.Asset{Key: petr, Class: domain.ClassStock, Name: "Petrobras"}, AsOf: asOf, Profile: p, Price: 38.5,
		Indicators: domain.IndicatorSet{Values: map[string]float64{"pe": 4.1, "roe": 0.22}},
		Valuation:  domain.ValuationCheap, Timing: domain.Timing{View: domain.TimingWait},
	}, nil
}
func (fakeAnalysis) Rank(context.Context, port.RankFilter) ([]port.RankedAsset, error) {
	return nil, nil
}
func (fakeAnalysis) Outlook(_ context.Context, p domain.RiskProfile) (domain.Outlook, error) {
	return scoring.OutlookFor(p, scoring.MacroContext{}), nil
}

// fakeLLM replies with each answer in turn and records the requests.
type fakeLLM struct {
	answers []string
	reqs    []port.LLMRequest
}

func (f *fakeLLM) Name() string { return "fake" }
func (f *fakeLLM) Generate(_ context.Context, r port.LLMRequest) (port.LLMResponse, error) {
	f.reqs = append(f.reqs, r)
	a := f.answers[0]
	f.answers = f.answers[1:]
	return port.LLMResponse{Text: a, Model: "fake-1", TokensIn: 100, TokensOut: 50}, nil
}

type connector struct{ llm *fakeLLM }

func (c connector) Connect(cred port.LLMCredential) (port.LLMProvider, error) {
	if cred.Provider != "fake" {
		return nil, domain.ErrInvalid
	}
	return c.llm, nil
}

const good = `{"summary":"PETR4 trades at a P/E of 4.1 with ROE of 22%.","bull_case":["Cheap at 4.1x earnings."],"bear_case":["State control."],
"valuation_view":"cheap","timing_view":"wait","fit_for_profile":"good","suggested_allocation_pct":{"min":2,"max":5},
"key_risks":["Oil prices."],"confidence":"medium","cited_data":["pe","roe"]}`

func run(t *testing.T, answers ...string) (domain.Report, *fakeLLM, *memory.AnalysisStore, error) {
	t.Helper()
	llm := &fakeLLM{answers: answers}
	store := memory.NewAnalysisStore()
	svc := report.New(report.Deps{Analysis: fakeAnalysis{}, News: memory.NewFeedStore(), Reports: store, LLM: connector{llm}},
		report.Options{Now: func() time.Time { return asOf.Add(24 * time.Hour) }})
	r, err := svc.Generate(context.Background(), port.ReportRequest{Asset: petr, Profile: domain.ProfileModerate,
		Credential: port.LLMCredential{Provider: "fake", APIKey: "sk-secret-1234"}})
	return r, llm, store, err
}

func TestGenerateGroundedFirstTime(t *testing.T) {
	r, llm, store, err := run(t, good)
	if err != nil {
		t.Fatal(err)
	}
	if len(llm.reqs) != 1 || r.Output.ValuationView != "cheap" || r.Output.AllocationMaxPct != 5 || len(r.Omitted) != 0 {
		t.Errorf("report = %+v (%d calls)", r, len(llm.reqs))
	}
	req := llm.reqs[0]
	if !strings.Contains(req.Prompt, `"pe": 4.1`) || !strings.Contains(req.System, "not financial advice") || len(req.JSONSchema) == 0 {
		t.Errorf("request = %+v", req)
	}
	if strings.Contains(req.Prompt, "sk-secret") {
		t.Error("the API key must never reach the prompt")
	}
	if got := store.Reports(); len(got) != 1 || got[0].Snapshot.Facts["pe"] != 4.1 || got[0].Provider != "fake" || r.ID != 1 {
		t.Errorf("stored = %+v", got)
	}
}

func TestGenerateRetriesThenSucceeds(t *testing.T) {
	invented := strings.Replace(good, "ROE of 22%", "ROE of 22% and 40% upside", 1)
	r, llm, _, err := run(t, invented, good)
	if err != nil {
		t.Fatal(err)
	}
	if len(llm.reqs) != 2 || !strings.Contains(llm.reqs[1].Prompt, "summary: numbers not in the data: 40%") {
		t.Fatalf("retry prompt = %q", llm.reqs[len(llm.reqs)-1].Prompt)
	}
	if r.TokensIn != 200 || len(r.Omitted) != 0 || !strings.Contains(r.Output.Summary, "22%") {
		t.Errorf("report = %+v", r)
	}
}

func TestGenerateOmitsWhatFailsTwice(t *testing.T) {
	bad := strings.Replace(good, `["Cheap at 4.1x earnings."]`, `["Cheap at 4.1x earnings.","Target of R$ 55.30."]`, 1)
	r, _, _, err := run(t, bad, bad)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Output.BullCase) != 1 || len(r.Omitted) != 1 || r.Omitted[0] != "bull_case[1]" {
		t.Errorf("report = %+v", r)
	}
}

func TestGenerateErrors(t *testing.T) {
	if _, _, _, err := run(t, "not json", "still not json"); err == nil || !strings.Contains(err.Error(), "no valid report") {
		t.Errorf("garbage twice err = %v", err)
	}
	svc := report.New(report.Deps{Analysis: fakeAnalysis{}, News: memory.NewFeedStore(), Reports: memory.NewAnalysisStore(), LLM: connector{&fakeLLM{}}}, report.Options{})
	if _, err := svc.Generate(context.Background(), port.ReportRequest{Asset: petr}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("no key err = %v", err)
	}
	_, err := svc.Generate(context.Background(), port.ReportRequest{Asset: domain.AssetKey{Market: domain.MarketB3, Symbol: "NOPE3"}, Credential: port.LLMCredential{Provider: "fake", APIKey: "k"}})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown asset err = %v", err)
	}
	if got := (port.LLMCredential{Provider: "anthropic", APIKey: "sk-ant-abcdef9876"}).String(); strings.Contains(got, "abcdef") || !strings.Contains(got, "9876") {
		t.Errorf("credential String() = %q", got)
	}
}
