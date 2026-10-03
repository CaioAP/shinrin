# Dev log 3: the analysis engine

Phase 3 of Shinrin: turn the data the pipeline collects into what an analyst
would conclude, first with plain rules, then optionally with an LLM that
explains those conclusions in prose. Notes for the blog post.

## What was built

- **Factor scores** for every stock, FII and REIT: valuation, quality,
  growth, momentum, income, risk and sentiment, each 0 to 100 against peers,
  each keeping the metrics behind it.
- **A composite per risk profile.** The same scores weighted differently for
  a conservative, moderate or aggressive investor.
- **Fair value ranges** with three classic models: the Graham number, the
  Bazin rule (popular in Brazil) and a five-year DCF on free cash flow.
- **Two views with reasons**: valuation (cheap, fair, expensive) and timing
  (accumulate, wait, avoid). Each comes with the list of rules that fired.
- **Allocation bands** per profile, leaning higher or lower with real
  interest rates in Brazil and the shape of the US yield curve.
- **AI reports** with the user's own key: an analyst-style note in a fixed
  JSON shape, with every number checked against the data the model was given.
- **API endpoints** for one asset's analysis, a ranked screener and the
  outlook, plus `shinrin report` on the command line.

## Decisions worth writing about

1. **Rank against peers, don't use thresholds.** "P/E under 10 is cheap" is
   wrong for a bank, a utility and a software company at the same time.
   Each metric is a percentile within the asset's market and sector (or the
   whole market when the sector has fewer than five members), turned around
   when lower is better. Thresholds remain only where "good" means something
   by itself: a payout ratio near 100% is risky however common it is.
2. **Higher is always better.** A risk score of 90 means *low* risk. One
   direction for every factor keeps the composite a simple weighted mean and
   keeps the UI from needing a legend for every number.
3. **Absent is not zero.** A factor with no data is left out, and the
   composite reports its *coverage*: the share of the profile's weights that
   had a score. A 75 built on 40% coverage deserves less trust than a 75 built
   on 90%, and the API says so.
4. **Store the scores, derive the rest.** Only factor scores need the whole
   universe at once (they are relative), so only they are stored, by the
   nightly `scoring` routine. The composite, fair values and views are cheap
   pure functions, recomputed on every read, so a different profile or a new
   Selic reading needs no batch run.
5. **One metric set does not fit all.** EV/EBITDA and net debt mean nothing
   for a bank, whose debt is its raw material, or for an FII. The scoring
   model is a Strategy picked per asset: stocks, financials and funds. Funds
   skip quality and growth until fund reports are ingested, rather than
   faking them.
6. **Ranges, never targets.** Every fair value is a low and a high with its
   assumptions attached (EPS, book value, discount rate). Graham's low end
   applies his one-third margin of safety, and the DCF has a pessimistic and
   an optimistic case.
7. **Timing is a vote of signals.** Trend (price and the 50-day average
   against the 200-day), MACD, RSI 70/30, the position in the 52-week range
   and the valuation view each add or subtract a weight. Technicals decide
   *when*; the factor scores decide *what*.
8. **The LLM can't invent numbers, and that is enforced.** The design
   promised it; `domain/report.Validate` makes it true. It pulls every
   figure out of the prose (it understands "22.3%", "4.1x", "R$ 453
   bilhões", "12,5") and matches each against the snapshot, allowing for
   rounding and fractions written as percentages. Phrases like "200-day" or
   "52 semanas" are not claims, so they pass. A failing answer goes back to
   the model once with the list of problems; whatever still fails is dropped
   and named, rather than rejecting a mostly good note.
9. **Bring your own key, never see it.** Keys live only in memory for one
   call. A credential prints as `…1234`, errors carry status codes rather than
   provider bodies on the Anthropic side, and the config's `String()` hides
   the key. Until accounts exist, reports run from the CLI with the
   operator's key; per-user encrypted keys and cost caps come with phase 4.
10. **Official SDK for Anthropic, plain HTTP for the rest.** The Anthropic
    adapter uses the Go SDK for structured output and the server-side
    refusal fallback. The OpenAI adapter speaks the Chat Completions wire
    format, which OpenRouter and many gateways share.

## What the first live run showed

Run on 2026-10-03 against a local Postgres with real B3 data (Ibovespa
members, COTAHIST prices from June 2024, B3 dividends, CVM filings, CVM
material facts, FRED):

- **All 76 Ibovespa members scored** in 62 ms. Valuation covered 73, quality
  67 (CVM skips banks), momentum, income and risk all 76.
- **Rankings differ by profile, as intended.** Conservative put CXSE3,
  ITSA4, ABEV3, BBSE3 and PETR4 on top. Aggressive moved PETR4 up to second
  on its momentum score (91).
- **PETR4 (moderate)**: composite 73.8 with 75% coverage. Graham range
  62.14 to 93.21 and Bazin 45.69 to 60.92 around a price of 51.17, so
  valuation "fair". Timing "wait": uptrend, but MACD below its signal line.
- **Gaps the run exposed:**
  - **B3 assets have no sector.** B3's index service doesn't send one, so
    every B3 asset is ranked market-wide and B3 banks can't be detected as
    financials. US assets get GICS sectors from the S&P 500 list. Next fix:
    read the sector from B3's company detail service.
  - **Sentiment scored only 1 of 76.** CVM headlines are formal
    ("Comunicado ao Mercado: Agenda da Semana") and carry no tone words.
    Finnhub's US headlines will do better. For B3, the material-fact
    *content* or the LLM inside a report is the real answer.
  - **Growth was empty** because the run started in June 2024. Growth needs
    two years of filings and the CAGR needs four, so a full backfill fixes
    it.
  - **No DCF for B3 yet**: BCB's API was still returning 502, so there is no
    Selic. The engine skipped the DCF with a note instead of guessing, which
    is the behaviour we wanted.
- **Not exercised live yet:** a real LLM call. The adapters are tested
  against recorded request and response shapes.

## Numbers

- About 4,000 lines of Go across `domain/scoring`, `domain/report`, three
  application services, two LLM adapters, Postgres storage and the HTTP
  endpoints. Every rule has a test with fixed numbers.
- First live scoring run: 76 assets in 62 ms. Fetching the B3 data from June
  2024 took 5.5 minutes, mostly COTAHIST downloads.
