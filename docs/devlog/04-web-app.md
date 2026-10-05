# Dev log 4: the web app

Phase 4 of Shinrin: put the pipeline and the analysis engine in front of a
person, with the "not financial advice" disclaimer everywhere a number could
be read as a call. Built in slices, each one a pull request with green CI.
Notes for the blog post.

## Slice 1: read views (scores, fair values, timing)

### What was built

- **Overview page**: the project story, the disclaimer, a macro strip (Selic,
  CDI, IPCA, USD/BRL, Fed funds, US 10-year, US CPI), the allocation bands for
  the chosen risk profile and the top five scores in each market.
- **Explore page**: the screener. Every scored asset ranked by the composite
  for the profile, with all seven factor scores, coverage and date, filtered
  by market and class. Filters live in the URL, so a screen can be shared.
- **Asset page**: composite score, valuation and timing views with the rules
  that fired, a data-gaps box, an adjusted price chart, factor scores that
  open into the metrics behind them ("why this rating?"), fair value ranges
  drawn against the price, the indicator table, dividends and JCP, and news.
- **Four new read endpoints** in Go (prices, dividends, news, macro) behind a
  new `MarketService` port, so the pages never compute anything themselves.
- **English and Portuguese** from day one, switchable in the header, with
  BRL and USD formatting that follows the language.

### Decisions worth writing about

1. **Show missing data, never hide it.** The data pipeline has real gaps
   today: no sectors for B3 stocks, almost no B3 sentiment, a short growth
   history, and no Selic while the Central Bank API is down. Each gap shows
   up where it matters: the macro strip says "not available yet" for Selic,
   CDI and the dollar instead of leaving them out; B3 stocks carry a yellow
   "Sector unknown" badge; unscored factors say "No data" rather than 0; the
   engine's own notes ("DCF skipped: no risk-free rate stored") are listed in
   a data-gaps box on the asset page. A blank is honest; a zero is a lie.
2. **The disclaimer is a component, not a footer only.** The footer is on
   every page, and every view that shows a score, fair value or timing view
   also opens with a warning box saying these are rule-based opinions, with
   the data's "as of" date. The Go API already puts the disclaimer in every
   analysis response, so no client can show a score without having it.
3. **The browser never talks to Go.** Pages call composables, composables
   call Nuxt server routes, server routes call Go. That keeps cookies on one
   origin for the accounts slice and keeps the API address private.
4. **Nuxt UI v4 over shadcn-vue.** It is a Nuxt module (auto-imported, SSR
   ready, dark mode included) and its table, cards, alerts and tabs cover the
   whole app. Fonts are system fonts and icons are bundled, so the build
   downloads nothing and the site makes no third-party requests.
5. **TradingView's lightweight-charts for prices.** Small, canvas based,
   built for financial series. The line is the close adjusted for dividends
   and splits (computed in Go with the same method the indicators use), so a
   dividend doesn't look like a crash.
6. **The risk profile is a cookie until accounts exist.** Changing it in the
   header recomputes every composite and band on the server; nothing is
   stored per user yet.
7. **Translate by code, not by text.** Each signal from the engine has a
   stable code (`rsi_overbought`, `us_curve_inverted`), so the web app
   translates it and falls back to the English message for any new code.

### Still to do in later slices

- Accounts (email and password, sessions), risk questionnaire, watchlists.
- Bring-your-own LLM key per user (encrypted), AI reports in the asset page.
- Data freshness / routine status page.
- Engine notes are English-only strings; give them codes like the signals so
  they can be translated.
- Deploy.
