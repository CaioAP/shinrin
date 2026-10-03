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

## Slice 2: accounts, risk profile and watchlists

### What was built

- **Sign-up and sign-in** with email and password. Sign-up requires ticking
  the "not financial advice" acknowledgement, as the design asked.
- **A six-question risk questionnaire** modeled on Brazilian suitability
  (horizon, goal, reaction to a 20% fall, experience, liquidity needs, share
  of savings). The result becomes the default profile for every score.
- **Watchlists**: create, rename, delete, add from any asset page, and a
  table of each asset's scores. The overview shows the first list.
- **Settings**: account details, what is stored, retake the questionnaire,
  and delete the account (password required, removes everything).

### Decisions worth writing about

1. **The browser never holds the session token.** Go issues it, the Nuxt
   server puts it in an HttpOnly cookie and forwards it as a bearer token.
   JavaScript on the page can't read it, so an XSS bug can't steal a session.
   CSRF is handled where the cookie lives: the Nuxt server refuses any
   state-changing request whose Origin isn't the site itself.
2. **Store a hash of the session, not the session.** Tokens are 256 random
   bits, so a plain SHA-256 is enough (passwords need argon2 because people
   pick guessable ones; random tokens have no dictionary to attack). A copy
   of the sessions table can't be replayed.
3. **Don't tell attackers which emails exist.** Wrong email and wrong
   password give the same answer and take the same time (an unknown email
   still runs one argon2 verification). Five failures lock that email for 15
   minutes.
4. **Ownership in every query.** Every watchlist call carries the user id
   down to SQL (`WHERE id = $2 AND user_id = $1`), so another user's list is
   simply "not found". There is no code path that loads a list first and
   checks the owner afterwards.
5. **The questionnaire is codes, not text.** Go defines the questions and
   answer codes and scores them; the web app owns the wording in both
   languages. Adding a translation never touches scoring.
6. **A hydration bug worth a paragraph.** An empty-string table header
   rendered as an empty text node on the server and nothing on the client,
   and Vue only says "hydration mismatch". Found it by diffing server HTML
   against the hydrated DOM, then removing columns one at a time.

## Slice 3: your own LLM key and AI reports in the app

Signed-in users can save their own LLM API key in Settings and get an
analyst-style report on any scored asset, written by their model from the
numbers Shinrin computed. The disclaimer sits at the top of every report,
followed by the model, the date, the data's as-of date and the profile.
The cited data is resolved from the stored snapshot, so a reader can check
every figure.

### Decisions worth writing about

1. **Envelope encryption, bound to the owner.** Each key is encrypted with
   its own random data key, and that data key is encrypted with a master key
   that lives only in the server's environment. The user id is the
   associated data on both layers, so copying a sealed key into someone
   else's row (a SQL bug, a bad restore) gives a decryption error rather than
   a working key. Rotating the master key means re-wrapping 48-byte data
   keys, not re-encrypting values.
2. **Write-only keys.** After saving, the API only ever shows the last four
   characters. Changing the model or the cap without retyping the key is
   allowed, but changing the provider or the base URL is not: otherwise
   someone with a stolen session could point the saved key at their own
   server and read it from the request.
3. **A user-supplied URL is an SSRF hole until proven otherwise.** The base
   URL (for OpenRouter and friends) must be public HTTPS, and the HTTP client
   checks the resolved IP when it dials, which is the only check DNS
   rebinding can't slip past. The cloud metadata address is refused by name
   as well as by range.
4. **Errors can leak secrets too.** Some providers quote part of the key in
   their 401 reply. Every error that may carry provider text goes through one
   redaction helper before it is logged or returned. Tests assert the key
   never appears in a response body or error, and a grep of the server logs
   after the browser run found no trace of it.
5. **The cap is the user's own guard.** Reports run on the user's money, so
   the cap is theirs to set; the server only enforces it, plus one report at
   a time per user. Counting money instead of reports needs per-model prices,
   which change too often to hard-code; that is a follow-up.
6. **No feature flag soup.** Without `SHINRIN_MASTER_KEY` the server says
   "AI reports are not enabled here" in the UI instead of silently storing
   keys in the clear or crashing.

Screenshots (sample report inserted for the screenshot, since the test key
was fake): `img/04-ai-settings.png`, `img/04-ai-report.png`.

### Still to do in later slices

- Run report generation as a background job, estimate cost per run.
- Data freshness / routine status page.
- Engine notes are English-only strings; give them codes like the signals so
  they can be translated.
- Deploy.
