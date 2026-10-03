// JSON contract of the Go API (backend/internal/adapter/in/httpapi/dto.go).
// Types in shared/ are auto-imported in both app/ and server/.

export type Market = 'B3' | 'US'

export type AssetClass = 'stock' | 'fii' | 'reit' | 'gov_bond' | 'index'

export interface Meta {
  name: string
  version: string
  disclaimer: string
}

export interface Asset {
  market: Market
  symbol: string
  class: AssetClass
  name: string
  sector?: string
  currency: 'BRL' | 'USD'
  indexMember: boolean
  active: boolean
}

export interface AssetFilter {
  market?: Market
  class?: AssetClass
  indexMember?: boolean
}

export interface ListResponse<T> {
  items: T[]
}

// Analysis engine (backend/internal/adapter/in/httpapi/dto.go). Every
// response carries the disclaimer; show it next to any score or view.

export type RiskProfile = 'conservative' | 'moderate' | 'aggressive'

export type Factor = 'valuation' | 'quality' | 'growth' | 'momentum' | 'income' | 'risk' | 'sentiment'

export interface ScoreInput {
  metric: string
  value: number
  /** The metric's own 0-100 contribution (peer percentile or absolute scale). */
  points: number
  weight: number
  peers?: number
}

export interface FactorScore {
  factor: Factor
  /** 0-100, higher is always better for the investor. */
  value: number
  peerGroup: string
  inputs: ScoreInput[]
}

export interface Signal {
  code: string
  message: string
  weight: number
  data?: Record<string, number>
}

export interface FairValue {
  method: 'graham' | 'bazin' | 'dcf'
  low: number
  high: number
  assumptions: Record<string, number>
}

export interface View<V extends string> {
  view: V
  signals: Signal[]
}

export interface Analysis {
  asset: Asset
  asOf: string
  profile: RiskProfile
  price: number
  composite: number
  /** Share (0-1) of the profile's factor weights that had a score. */
  coverage: number
  factors: FactorScore[]
  indicators: Record<string, number>
  fairValues: FairValue[]
  valuation: View<'cheap' | 'fair' | 'expensive'>
  timing: View<'accumulate' | 'wait' | 'avoid'>
  notes: string[]
  disclaimer: string
}

export interface RankingFilter {
  market?: Market
  class?: AssetClass
  profile?: RiskProfile
  limit?: number
}

export interface RankedAsset {
  asset: Asset
  asOf: string
  composite: number
  coverage: number
  factors: Partial<Record<Factor, number>>
}

export interface Ranking {
  profile: RiskProfile
  items: RankedAsset[]
  disclaimer: string
}

export interface AllocationBand {
  class: 'fixed_income' | 'stocks' | 'real_estate'
  min: number
  max: number
  lean: 'low' | 'mid' | 'high'
}

export interface Outlook {
  profile: RiskProfile
  asOf?: string
  bands: AllocationBand[]
  signals: Signal[]
  notes: string[]
  disclaimer: string
}

// Market data (backend/internal/adapter/in/httpapi/market.go).

export type PriceRange = '1m' | '3m' | '6m' | '1y' | '5y' | 'max'

export interface PriceBar {
  date: string
  open: number
  high: number
  low: number
  close: number
  /** Close adjusted for dividends and splits. */
  adjClose: number
  volume: number
}

export interface PriceHistory {
  range: PriceRange
  source?: string
  items: PriceBar[]
}

export interface Dividend {
  exDate: string
  type: 'dividend' | 'jcp'
  /** Cash per share, in the asset's currency. */
  value: number
  source: string
}

export interface NewsItem {
  url: string
  title: string
  summary?: string
  lang?: string
  source: string
  publishedAt: string
}

export type MacroCode = 'selic' | 'cdi' | 'ipca_12m' | 'usdbrl' | 'fed_funds' | 'ust_10y' | 'us_cpi_12m'

export interface MacroIndicator {
  code: MacroCode
  value: number
  unit: 'pct_year' | 'brl_per_usd'
  asOf: string
  source: string
}

export interface MacroStrip {
  items: MacroIndicator[]
  /** Indicators that could not be computed from stored data. */
  missing: MacroCode[]
}

// Accounts and watchlists (backend/internal/adapter/in/httpapi/auth.go,
// watchlists.go). The session token never reaches the browser: Nuxt keeps
// it in an HttpOnly cookie.

export interface User {
  id: number
  email: string
  createdAt: string
  profile?: RiskProfile
  profileAnswers?: Record<string, string>
  profileUpdatedAt?: string
}

/** Server-side only: what Go returns on sign-in. */
export interface Session {
  user: User
  token: string
  expiresAt: string
}

export interface Question {
  id: string
  /** Answer codes from the most cautious to the boldest. */
  answers: string[]
}

export interface AssetKey {
  market: Market
  symbol: string
}

export interface Watchlist {
  id: number
  name: string
  assets: AssetKey[]
}

export interface WatchlistEntry {
  asset: Asset
  /** null until the asset has been scored. */
  scores: RankedAsset | null
}

export interface WatchlistEntries {
  id: number
  name: string
  profile: RiskProfile
  items: WatchlistEntry[]
  disclaimer: string
}

// Bring-your-own LLM key and AI reports (backend/internal/adapter/in/httpapi/reports.go).
// The API key is write-only: it is sent once on save and never comes back.

export type LLMProvider = 'anthropic' | 'openai'

export interface LLMUsage {
  used: number
  cap: number
  /** First day of the current month (UTC), when the count resets. */
  since: string
}

export interface LLMSettings {
  /** false when the server cannot store keys (no master key configured). */
  available: boolean
  configured: boolean
  provider?: LLMProvider
  model?: string
  baseUrl?: string
  /** The last four characters of the saved key, e.g. "…a1b2". */
  keyHint?: string
  monthlyCap?: number
  updatedAt?: string
  usage: LLMUsage
}

/** What the settings form sends. An empty apiKey keeps the saved key. */
export interface LLMSettingsInput {
  provider: LLMProvider
  model: string
  baseUrl: string
  apiKey: string
  monthlyCap: number
}

export interface ReportOutput {
  summary: string
  bullCase: string[]
  bearCase: string[]
  valuationView: 'cheap' | 'fair' | 'expensive'
  timingView: 'accumulate' | 'wait' | 'avoid'
  fitForProfile: 'good' | 'partial' | 'poor'
  suggestedAllocationPct: { min: number, max: number }
  keyRisks: string[]
  confidence: 'low' | 'medium' | 'high'
}

/** A piece of data the report cites, resolved from its input snapshot. */
export interface CitedData {
  key: string
  value?: number
  text?: string
}

export interface AIReport {
  id: number
  asset: AssetKey
  profile: RiskProfile
  /** The data's as-of date, not when the report was written. */
  asOf: string
  createdAt: string
  provider: string
  model: string
  tokensIn: number
  tokensOut: number
  output: ReportOutput
  cited: CitedData[]
  /** Sections dropped because they cited numbers the data does not contain. */
  omitted: string[]
  disclaimer: string
}
