# Shinrin: data source comparison (US + B3)

Researched 2026-10-01. Limits change often, so recheck each provider's pricing page before building against it.

## The short version

| Need | US | Brazil (B3) | Cost |
|---|---|---|---|
| Daily price history (for technicals) | Tiingo (EOD) or Financial Modeling Prep | **B3 COTAHIST files** (official, every ticker since 1986) | Free |
| Near-live quotes | Finnhub | brapi.dev (free tier, 30 min delay) | Free |
| Fundamentals (income, balance sheet, cash flow) | **SEC EDGAR XBRL API** (official) | **CVM Dados Abertos** (DFP annual + ITR quarterly) | Free |
| Company news | Finnhub company news | CVM IPE filings (fatos relevantes) + news RSS feeds | Free |
| Macro context | FRED (Fed rates, CPI) | **BCB SGS / Focus** (Selic, IPCA, PTAX, market forecasts) | Free |

The pattern: use official government and exchange sources for the heavy data (history and fundamentals), because they are free, unlimited in practice and have no redistribution worries. Use commercial APIs only for the "fresh" layer (live quotes, news), where rate limits are small but fine for a personal watchlist.

## Market data providers

### Brazil

**B3 COTAHIST (Séries Históricas)**
- Official daily bulletin of every B3 trade, downloadable as yearly, monthly or daily zip files with a fixed-width text layout.
- Free, no key, no rate limit. Ideal for a nightly job.
- Caveat: prices are not adjusted for dividends or splits. Shinrin must adjust them itself using corporate action data (CVM or brapi dividends) before computing returns or long moving averages.
- Links: [Cotações históricas](https://www.b3.com.br/pt_br/market-data-e-indices/servicos-de-dados/market-data/historico/mercado-a-vista/cotacoes-historicas/), [layout PDF](https://www.b3.com.br/data/files/33/67/B9/50/D84057102C784E47AC094EA8/SeriesHistoricas_Layout.pdf)

**CVM Dados Abertos**
- Official filings of every listed Brazilian company: DFP (annual statements), ITR (quarterly), FCA/FRE (registration data, share counts), IPE (material facts and announcements).
- Free CSV/zip, updated as companies file. This is the fundamentals backbone for B3.
- Caveat: raw accounting lines (CD_CONTA codes) need mapping to standard metrics, and banks/insurers use different charts of accounts.
- Links: [DFP](https://dados.cvm.gov.br/dataset/groups/cia_aberta-doc-dfp), [IPE](https://dados.cvm.gov.br/dataset/groups/cia_aberta-doc-ipe)

**brapi.dev**
- Brazilian REST API for B3 quotes, dividends and fundamentals.
- Free: 15,000 requests/month, quotes refreshed every 30 min, only 3 months of history, no fundamentals. Free plan is meant for "personal and academic" use.
- Pro: R$ 139.99/month for 500k requests, 10+ years history, quarterly fundamentals since 2009, 5 min refresh.
- Terms forbid redistributing or reselling the data, or building a proxy over it. Showing your own analysis in your own app is fine; exposing raw brapi data to the public is the grey zone.
- Links: [pricing](https://brapi.dev/pricing.md), [terms](https://brapi.dev/terms-of-use)

**Banco Central do Brasil (SGS, Focus, PTAX)**
- Selic (series 432), IPCA (433), USD/BRL, plus Focus market expectations. Needed for discount rates, real returns and macro context.
- Free JSON API. Gotchas: daily series reject windows over 10 years (HTTP 406), and IPCA must be compounded, not summed.
- Link: [SGS pitfalls write-up](https://www.tabnews.com.br/SidneyBissoli/series-do-banco-central-como-consultar-o-sgs-a-focus-e-a-ptax-sem-cair-nas-armadilhas)

### United States

**SEC EDGAR (companyfacts / XBRL frames API)**
- Official structured fundamentals for every US filer, free, needs only a User-Agent header, limit ~10 requests/second.
- Same mapping caveat as CVM (XBRL tags vary between companies).

**Finnhub**
- Free: about 60 calls/minute, real-time US quotes, company news, market news, basic financials.
- Free tier is personal, non-commercial only. Historical candles are a paid feature, so pair it with an EOD source.
- Link: [pricing summary](https://apicostcalc.com/finnhub.html)

**Financial Modeling Prep**
- Free: 250 calls/day, US EOD history, profiles, statements. Good backup for US fundamentals already normalized.

**Tiingo**
- Free tier with hourly/daily request caps and a monthly unique-symbol cap (exact numbers to confirm on their docs). Long, clean, split-adjusted US EOD history.

**Alpha Vantage**
- Free tier is now 25 requests/day. Too small to rely on; skip.

Comparison source for US free tiers: [QVeris comparison, Aug 2026](https://qveris.ai/guides/stock-api-free-comparison/)

## News

- **US:** Finnhub company news (headline, summary, source, URL) per ticker.
- **B3:** CVM IPE dataset gives every fato relevante and comunicado ao mercado, which is the most decision-relevant "news" for Brazilian stocks. Supplement with RSS feeds from Brazilian financial outlets (for example InfoMoney, Money Times, Valor) or Google News RSS queries by ticker.
- **Rule for news storage:** keep headline, link, source, timestamp and our own derived sentiment. Don't store or republish full article text, which is copyrighted.

## Unofficial sources to avoid as a foundation

Yahoo Finance (via yfinance-style scraping) covers both markets (`PETR4.SA`) and is popular in hobby projects, but it has no official API, its terms prohibit this use, and it breaks without notice. Fine for a quick prototype, not for something you will show publicly.

## Display terms

Free tiers from Finnhub and brapi are for personal use. Since Shinrin is a personal tool that also appears in your portfolio, the safe approach is:
1. The public demo shows our computed outputs (scores, ratios, charts we render), not raw data feeds.
2. Each page credits its data source.
3. If the demo ever gets real traffic or charges money, move to paid tiers.
4. Official sources (B3, CVM, SEC, BCB) carry no such restriction.

## Other asset classes (added 2026-10-01)

| Asset | Brazil source | US equivalent and source | Free? | Notes |
|---|---|---|---|---|
| Government bonds | **Tesouro Direto**: Tesouro Transparente CSV with daily price and rate history for every title | **Treasuries**: Treasury Fiscal Data API / FRED daily yield curve; TreasuryDirect for I bonds | Yes | Daily bulk files, almost no API calls. brapi's Tesouro endpoint is blocked on its free plan, so use the official CSV. |
| Bank fixed income | **CDB, RDB, LCI, LCA**: no public feed of offers, rates differ per bank and broker | **CDs**: FDIC publishes monthly national average rates; no feed of individual offers | Partly | Model as a calculator: user enters the offer (e.g. 110% of CDI, 2 years) and Shinrin projects it using CDI (BCB SGS series 12), Selic and IPCA, with IR regressivo tax (LCI/LCA exempt) and FGC/FDIC limits. |
| Real estate funds | **FII**: prices in B3 COTAHIST; monthly reports with income distributed, NAV and vacancy in CVM `fii-doc-inf_mensal` | **REITs**: normal stock tickers, so the stock pipeline and SEC data cover them | Yes | FII and REIT analysis uses different metrics (dividend yield, P/VP, FFO) from stocks. |
| Dividends | Stocks: weakest spot. brapi free has "limited" dividends; CVM filings contain them but are hard to parse. FIIs: CVM monthly reports. | Tiingo EOD includes dividend and split columns; FMP free has a dividends endpoint | Mostly | Also needed to adjust COTAHIST prices. Upgrading to brapi Pro later fixes B3 stock dividends cleanly. |

Links: [Tesouro Direto API guide (brapi blog)](https://brapi.dev/blog/api-tesouro-direto-brasil-como-consultar-2026), [CVM FII monthly reports](https://dados.cvm.gov.br/dataset/groups/fii-doc-inf_mensal), [free yield curve APIs](https://qveris.ai/guides/free-yield-curve-api/)
