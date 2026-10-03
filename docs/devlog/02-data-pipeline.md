# Dev log 2: the data pipeline

Phase 2 of Shinrin: get prices, corporate actions and fundamentals for the
Ibovespa and the S&P 500 into Postgres on a schedule, and turn them into the
standard indicators an analyst screens with. Notes for the blog post.

## What was built

- **Sources, one adapter each**: B3 COTAHIST files (B3 prices), B3's
  listed-company service (Ibovespa members, B3 dividends, JCP and splits), CVM
  DFP/ITR (B3 fundamentals), SEC EDGAR companyfacts (US fundamentals), Tiingo
  (US prices, dividends and splits) and the community S&P 500 list.
- **Ingestion service** that decides *what* to fetch (only missing days, only
  tracked assets) while adapters decide *how*.
- **Storage** in Postgres with goose migrations, **scheduling** with River
  (cron per routine, one queue per provider, retries, one run at a time).
- **Indicators** as pure functions: SMA 50/200, RSI 14, MACD, 1/3/6/12-month
  returns, volatility, max drawdown, 52-week range, market cap, EV, P/E, P/B,
  P/S, EV/EBITDA, EV/EBIT, dividend yield, FCF yield, ROE, margins, net
  debt/EBITDA.

## Decisions worth writing about

1. **Bulk files beat per-ticker APIs for B3.** One COTAHIST file has every
   ticker for a day, month or year. So B3 got its own port,
   `MarketPriceSource`, instead of being forced through a per-asset
   `PriceSource`. The adapter picks the fewest files: yearly for past years,
   monthly for past months, daily for this month. A first backfill is about
   sixteen yearly downloads, processed one year at a time to keep memory flat.
2. **Adjust prices ourselves.** COTAHIST prices are raw. Tiingo publishes an
   adjusted close, but that whole history is rewritten every time a dividend is
   paid, so a stored copy goes stale the next day. Shinrin stores raw closes
   plus corporate actions and back-adjusts at analysis time
   (`domain.AdjustCloses`), the same way for both markets.
3. **The fourth quarter does not exist.** Companies file Q1 to Q3 and an
   annual report. Trailing-twelve-month numbers need Q4 = FY - Q1 - Q2 - Q3,
   which lives in the domain (`Financials.TTM`) so every source benefits.
   Brazilian ITRs make it worse: cash flow statements are only year-to-date,
   so the CVM adapter derives quarters from consecutive YTD values.
4. **Sources are strategies passed per routine.** `ingest.Service` holds only
   the stores; each routine is built with its source
   (`PricesRoutine("us_prices_eod", US, tiingo, tiingo)`), so B3 and US share
   all the ingestion code.
5. **Politeness is a decorator, not adapter code.** User-Agent, token-bucket
   rate limit and retry with backoff are `http.RoundTripper` decorators in
   `internal/httpx`, configured per provider in `wire.go`. Adapters receive a
   plain `*http.Client` and are tested with `httptest`.
6. **Failures are per asset.** One delisted ticker must not stop 500 others.
   A source answers `domain.ErrNotFound` for what it does not cover (an FII
   has no DFP, a bank's chart of accounts is not mapped); everything else is
   counted, logged, and returned so River retries the routine. Because every
   sync is incremental, a retry only fetches what is still missing.

## Difficulties

- **No free S&P 500 list.** S&P does not publish it. Using the
  `datasets/s-and-p-500-companies` CSV (from Wikipedia), refreshed weekly, and
  refusing an empty list so a broken source cannot un-track the market.
- **Tiingo's free tier is tight**: about 50 requests an hour, so the first S&P
  500 backfill takes a working day. The adapter makes one request per ticker
  for both prices and dividends (it keeps the response for the second call),
  and the routine timeout is generous because it resumes where it stopped.
- **CVM is Latin-1, semicolon-separated, versioned and duplicated.** Each
  filing repeats last year's numbers ("PENÚLTIMO"), resubmissions bump
  `VERSAO`, consolidated and individual statements coexist, and values can be
  in thousands (`ESCALA_MOEDA = MIL`). Depreciation, capex and dividends paid
  have company-specific sub-accounts, matched by description.
- **Banks don't have revenue.** CVM banks report "Receitas da Intermediação
  Financeira"; EBITDA and gross debt are meaningless there. Skipped for now,
  flagged as an open item.
- **Market cap needs a per-share price.** B3 units (TAEE11) bundle ordinary
  and preferred shares, so unit price times the company's share count is off
  by the bundle size. Units get technicals and margins but no P/E or market
  cap until the bundle composition is modelled. Multi-class US companies
  (GOOG/GOOGL) have a similar, smaller issue with the reported share count.
- **B3 dividends live in an undocumented service.** It is the exchange's own
  data and free, so it wins over brapi's limited free tier, but the parser is
  strict on purpose.
- **XBRL tags vary per company.** Revenue alone has four common tags. The SEC
  adapter tries them in order of preference per period, keeps 3- and 12-month
  durations only, and lets the latest filing win so restatements apply.
- **Go 1.26.** Current pgx, River and goose releases require it, so the
  module moved from 1.24.
- **Built offline.** The build environment could not reach B3, CVM, SEC or
  Tiingo, so every adapter is tested against fixtures written in the
  provider's documented format. The first live run is the real test.

## Not in this phase yet

Intraday quotes (Finnhub, brapi), news (Finnhub, CVM IPE, RSS), macro (BCB,
FRED), bonds (Tesouro, Treasury), FII monthly reports, bank fundamentals, and
API endpoints to read prices and indicators.
