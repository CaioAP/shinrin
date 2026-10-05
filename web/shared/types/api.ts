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
