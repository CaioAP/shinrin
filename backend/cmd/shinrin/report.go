package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// runReport writes one AI report with the operator's own LLM key and prints
// it as JSON. Web users will generate reports with their own stored keys
// once accounts exist; this command is how the engine is exercised until
// then.
func runReport(ctx context.Context, c *container, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	profile := fs.String("profile", "moderate", "risk profile: conservative, moderate or aggressive")
	lang := fs.String("lang", "en", "report language: en or pt-BR")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: shinrin report [-profile p] [-lang l] <market> <symbol>")
	}
	if c.pool == nil {
		return errNoDatabase // the analysis reads stored scores
	}
	if c.cfg.LLM.APIKey == "" {
		return errors.New("SHINRIN_LLM_API_KEY is required (your own Anthropic or OpenAI-compatible key)")
	}
	market, err := domain.ParseMarket(fs.Arg(0))
	if err != nil {
		return err
	}
	symbol, err := domain.NewSymbol(fs.Arg(1))
	if err != nil {
		return err
	}
	p, err := domain.ParseRiskProfile(*profile)
	if err != nil {
		return err
	}
	r, err := c.report.Generate(ctx, port.ReportRequest{
		Asset: domain.AssetKey{Market: market, Symbol: symbol}, Profile: p, Lang: *lang,
		Credential: port.LLMCredential{Provider: c.cfg.LLM.Provider, Model: c.cfg.LLM.Model, APIKey: c.cfg.LLM.APIKey, BaseURL: c.cfg.LLM.BaseURL},
	})
	if err != nil {
		return err
	}
	c.log.Info("report written", "id", r.ID, "asset", r.Asset.String(), "model", r.Model, "tokens_in", r.TokensIn, "tokens_out", r.TokensOut, "omitted", r.Omitted)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"disclaimer": domain.Disclaimer,
		"id":         r.ID,
		"asset":      r.Asset.String(),
		"profile":    r.Profile,
		"asOf":       r.AsOf.Format("2006-01-02"),
		"model":      r.Model,
		"report":     reportView(r.Output),
		"omitted":    r.Omitted,
		"tokens":     fmt.Sprintf("%d in, %d out", r.TokensIn, r.TokensOut),
	})
}

func reportView(o domain.ReportOutput) map[string]any {
	return map[string]any{
		"summary": o.Summary, "bull_case": o.BullCase, "bear_case": o.BearCase,
		"valuation_view": o.ValuationView, "timing_view": o.TimingView, "fit_for_profile": o.FitForProfile,
		"suggested_allocation_pct": map[string]float64{"min": o.AllocationMinPct, "max": o.AllocationMaxPct},
		"key_risks":                o.KeyRisks, "confidence": o.Confidence, "cited_data": o.CitedData,
	}
}
