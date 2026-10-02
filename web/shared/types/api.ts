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
