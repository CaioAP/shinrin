// Package report writes AI reports (docs/design.md, section 9): it builds
// the input snapshot from the deterministic analysis, asks the user's own
// LLM for an analyst-style note in a fixed JSON shape, rejects figures the
// snapshot does not contain, retries once, and stores the result with its
// snapshot. The LLM adds narrative and judgment; it never supplies numbers.
package report

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/domain/report"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Deps are the ports the service uses.
type Deps struct {
	Analysis port.AnalysisService
	News     port.NewsReader
	Reports  port.ReportWriter
	LLM      port.LLMConnector
}

// Options tune the service; zero values take defaults.
type Options struct {
	Now          func() time.Time
	Logger       *slog.Logger
	MaxHeadlines int // default 10
	MaxTokens    int // default 16000
}

// Service implements port.ReportService.
type Service struct {
	d   Deps
	opt Options
}

var _ port.ReportService = (*Service)(nil)

// New builds the service.
func New(d Deps, opt Options) *Service {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.Logger == nil {
		opt.Logger = slog.New(slog.DiscardHandler)
	}
	if opt.MaxHeadlines == 0 {
		opt.MaxHeadlines = 10
	}
	if opt.MaxTokens == 0 {
		opt.MaxTokens = 16000
	}
	return &Service{d: d, opt: opt}
}

const newsWindow = 30 * 24 * time.Hour

// Generate implements port.ReportService.
func (s *Service) Generate(ctx context.Context, req port.ReportRequest) (domain.Report, error) {
	if req.Credential.APIKey == "" || req.Credential.Provider == "" {
		return domain.Report{}, fmt.Errorf("%w: an LLM provider and API key are required", domain.ErrInvalid)
	}
	a, err := s.d.Analysis.Analyze(ctx, req.Asset, req.Profile)
	if err != nil {
		return domain.Report{}, fmt.Errorf("analyze %s: %w", req.Asset, err)
	}
	o, err := s.d.Analysis.Outlook(ctx, req.Profile)
	if err != nil {
		return domain.Report{}, fmt.Errorf("outlook: %w", err)
	}
	news, err := s.d.News.NewsFor(ctx, req.Asset, s.opt.Now().Add(-newsWindow), s.opt.MaxHeadlines)
	if err != nil {
		return domain.Report{}, fmt.Errorf("news of %s: %w", req.Asset, err)
	}
	snap := report.NewSnapshot(a, o, news, s.opt.MaxHeadlines)

	llm, err := s.d.LLM.Connect(req.Credential)
	if err != nil {
		return domain.Report{}, err
	}
	first := userPrompt(snap, req.Lang)
	call := func(prompt string) (port.LLMResponse, error) {
		return llm.Generate(ctx, port.LLMRequest{Model: req.Credential.Model, System: systemPrompt, Prompt: prompt, MaxTokens: s.opt.MaxTokens, JSONSchema: schema})
	}

	r := domain.Report{Kind: domain.ReportKindAsset, Asset: req.Asset, Profile: req.Profile, AsOf: a.AsOf, Snapshot: snap, Provider: llm.Name()}
	resp, err := call(first)
	if err != nil {
		return domain.Report{}, fmt.Errorf("%s: %w", llm.Name(), err)
	}
	r.Model, r.TokensIn, r.TokensOut = resp.Model, resp.TokensIn, resp.TokensOut
	out, problems := check(resp.Text, snap)

	if len(problems) > 0 {
		s.opt.Logger.Info("report failed validation, retrying", "asset", req.Asset, "problems", len(problems))
		resp, err = call(retryPrompt(first, resp.Text, problems))
		if err != nil {
			return domain.Report{}, fmt.Errorf("%s retry: %w", llm.Name(), err)
		}
		r.TokensIn += resp.TokensIn
		r.TokensOut += resp.TokensOut
		var parsed bool
		out, parsed = parseAndOmit(resp.Text, snap, &r)
		if !parsed {
			return domain.Report{}, fmt.Errorf("%w: %s returned no valid report after a retry", errLLMOutput, llm.Name())
		}
	}
	r.Output = out
	r.CreatedAt = s.opt.Now()
	if r.ID, err = s.d.Reports.SaveReport(ctx, r); err != nil {
		return domain.Report{}, fmt.Errorf("store report: %w", err)
	}
	return r, nil
}

var errLLMOutput = errors.New("invalid LLM output")

// check parses and validates one answer, returning the problems to send
// back on a retry.
func check(text string, snap domain.ReportSnapshot) (domain.ReportOutput, []string) {
	out, err := parseOutput(text)
	if err != nil {
		return domain.ReportOutput{}, []string{err.Error()}
	}
	return out, issueStrings(report.Validate(out, snap))
}

// parseAndOmit accepts the retried answer, removing whatever still fails.
func parseAndOmit(text string, snap domain.ReportSnapshot, r *domain.Report) (domain.ReportOutput, bool) {
	out, err := parseOutput(text)
	if err != nil {
		return domain.ReportOutput{}, false
	}
	out, r.Omitted = report.Omit(out, report.Validate(out, snap))
	return out, true
}
