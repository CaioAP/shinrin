package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/in/jobs"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

const (
	brt = "CRON_TZ=America/Sao_Paulo "
	et  = "CRON_TZ=America/New_York "
)

// routines is the registry of scheduled work (docs/design.md, section 7).
// Each entry pairs a routine from an application service with the source
// adapters it uses, a cron schedule and a queue; one queue per provider keeps
// its rate limit to one job at a time. Routines whose provider is not
// configured are left out.
func (c *container) routines() []jobs.Entry {
	b3Files, b3API, cvmClient := c.b3Files(), c.b3API(), c.cvm()
	entries := []jobs.Entry{
		{Routine: c.ingest.UniverseRoutine("ibov_members", b3API, domain.IndexIbovespa), Schedule: brt + "0 6 * * 1", Queue: "b3"},
		{Routine: c.ingest.UniverseRoutine("sp500_members", c.sp500(), domain.IndexSP500), Schedule: et + "0 6 * * 1", Queue: "github"},
		{Routine: c.ingest.MarketPricesRoutine("b3_prices_eod", b3Files), Schedule: brt + "30 20 * * 1-5", Queue: "b3"},
		{Routine: c.ingest.CorporateActionsRoutine("b3_corporate_actions", domain.MarketB3, b3API), Schedule: brt + "0 21 * * 1-5", Queue: "b3"},
		{Routine: c.ingest.FundamentalsRoutine("cvm_fundamentals", domain.MarketB3, cvmClient), Schedule: brt + "0 7 * * *", Queue: "cvm", Timeout: 4 * time.Hour},
		// After the last EOD sync of both markets.
		{Routine: c.analytics.Routine("indicators", domain.MarketB3, domain.MarketUS), Schedule: brt + "30 23 * * 1-5", Queue: "internal"},
		// Scores rank each asset against its peers, so they run over the
		// whole universe once indicators are fresh. Same queue: River runs
		// one job at a time per queue, so this waits for indicators.
		{Routine: c.scoring.Routine("scoring", domain.MarketB3, domain.MarketUS), Schedule: brt + "45 23 * * 1-5", Queue: "internal"},
		// Material facts: own queue so news is not stuck behind a fundamentals
		// backfill; the shared client still holds CVM's rate limit. CVM
		// republishes the IPE files weekly, so read two weeks back and poll
		// twice a day rather than every few minutes.
		{Routine: c.ingest.NewsRoutine("cvm_news", cvmClient, 14*24*time.Hour), Schedule: brt + "0 8,20 * * *", Queue: "cvm_news"},
		{Routine: c.ingest.MacroRoutine("macro_br", c.bcb()), Schedule: brt + "0 9,19 * * *", Queue: "bcb"},
		{Routine: c.ingest.BondsRoutine("tesouro_bonds", c.tesouro()), Schedule: brt + "0 10,19 * * 1-5", Queue: "tesouro"},
	}
	if t := c.tiingo(); t != nil {
		entries = append(entries, jobs.Entry{Routine: c.ingest.PricesRoutine("us_prices_eod", domain.MarketUS, t, t), Schedule: et + "30 18 * * 1-5", Queue: "tiingo", Timeout: 12 * time.Hour})
	} else {
		c.log.Warn("SHINRIN_TIINGO_TOKEN not set: US prices are disabled")
	}
	if s := c.sec(); s != nil {
		entries = append(entries, jobs.Entry{Routine: c.ingest.FundamentalsRoutine("sec_fundamentals", domain.MarketUS, s), Schedule: et + "0 7 * * *", Queue: "sec", Timeout: 4 * time.Hour})
	} else {
		c.log.Warn("SHINRIN_SEC_USER_AGENT not set: US fundamentals are disabled")
	}
	if f := c.finnhub(); f != nil {
		// One queue: both routines spend the same per-minute quota.
		entries = append(entries,
			jobs.Entry{Routine: c.ingest.QuotesRoutine("quotes_us", domain.MarketUS, f), Schedule: et + "*/30 9-16 * * 1-5", Queue: "finnhub", Timeout: 30 * time.Minute},
			jobs.Entry{Routine: c.ingest.CompanyNewsRoutine("news_us", domain.MarketUS, f), Schedule: et + "15 7,12,18 * * *", Queue: "finnhub", Timeout: time.Hour},
		)
	} else {
		c.log.Warn("SHINRIN_FINNHUB_TOKEN not set: US quotes and news are disabled")
	}
	if b := c.brapi(); b != nil {
		entries = append(entries, jobs.Entry{Routine: c.ingest.QuotesRoutine("quotes_b3", domain.MarketB3, b), Schedule: brt + "0 10,12,14,16,18 * * 1-5", Queue: "brapi"})
	} else {
		c.log.Warn("SHINRIN_BRAPI_TOKEN not set: B3 intraday quotes are disabled")
	}
	if f := c.fred(); f != nil {
		entries = append(entries, jobs.Entry{Routine: c.ingest.MacroRoutine("macro_us", f), Schedule: et + "0 9,18 * * *", Queue: "fred"})
	} else {
		c.log.Warn("SHINRIN_FRED_API_KEY not set: US macro series are disabled")
	}
	return entries
}

// runWorker schedules every routine with River until ctx is cancelled.
func runWorker(ctx context.Context, c *container) error {
	if c.pool == nil {
		return errNoDatabase
	}
	entries := c.routines()
	sched, err := jobs.New(c.pool, entries, c.log)
	if err != nil {
		return err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Routine.Name()
	}
	c.log.Info("worker started", "routines", names)
	return sched.Run(ctx, c.cfg.ShutdownTimeout)
}

// runRoutines runs the named routines once, in order, in this process. It is
// how a first backfill is done (`shinrin run ibov_members b3_prices_eod`)
// and how a routine is debugged without the scheduler.
func runRoutines(ctx context.Context, c *container, names []string) error {
	if c.pool == nil {
		return errNoDatabase // results would vanish with the process
	}
	byName := map[string]jobs.Entry{}
	for _, e := range c.routines() {
		byName[e.Routine.Name()] = e
	}
	if len(names) == 0 {
		return fmt.Errorf("name at least one routine: %s", strings.Join(sortedKeys(byName), ", "))
	}
	for _, n := range names {
		e, ok := byName[n]
		if !ok {
			return fmt.Errorf("unknown routine %q (have %s)", n, strings.Join(sortedKeys(byName), ", "))
		}
		start := time.Now()
		c.log.Info("routine started", "routine", n)
		if err := e.Routine.Run(ctx); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		c.log.Info("routine finished", "routine", n, "took", time.Since(start).Round(time.Millisecond).String())
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
