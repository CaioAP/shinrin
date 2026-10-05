// How each indicator from the Go engine (backend/internal/domain/indicators
// and domain/scoring/metrics.go) is displayed. Ratios arrive as fractions.

type MetricKind = 'fraction' | 'multiple' | 'money' | 'compactMoney' | 'number'

const kinds: Record<string, MetricKind> = {
  close: 'money',
  sma_50: 'money',
  sma_200: 'money',
  high_52w: 'money',
  low_52w: 'money',
  market_cap: 'compactMoney',
  enterprise_value: 'compactMoney',
  pe: 'multiple',
  pb: 'multiple',
  ps: 'multiple',
  ev_ebitda: 'multiple',
  ev_ebit: 'multiple',
  net_debt_ebitda: 'multiple',
  return_1m: 'fraction',
  return_3m: 'fraction',
  return_6m: 'fraction',
  return_12m: 'fraction',
  volatility_1y: 'fraction',
  max_drawdown_1y: 'fraction',
  dividend_yield: 'fraction',
  fcf_yield: 'fraction',
  roe: 'fraction',
  net_margin: 'fraction',
  ebit_margin: 'fraction',
  revenue_growth_1y: 'fraction',
  earnings_growth_1y: 'fraction',
  revenue_cagr_3y: 'fraction',
  price_vs_sma_200: 'fraction',
  price_vs_sma_50: 'fraction',
  from_high_52w: 'fraction',
  from_low_52w: 'fraction',
  payout_ratio: 'fraction',
}

/** Display order of the indicator grid, grouped as analysts read them. */
export const indicatorGroups: string[][] = [
  ['close', 'market_cap', 'enterprise_value', 'high_52w', 'low_52w'],
  ['pe', 'pb', 'ps', 'ev_ebitda', 'ev_ebit', 'dividend_yield', 'fcf_yield'],
  ['roe', 'net_margin', 'ebit_margin', 'net_debt_ebitda'],
  ['revenue_growth_1y', 'earnings_growth_1y', 'revenue_cagr_3y'],
  ['return_1m', 'return_3m', 'return_6m', 'return_12m', 'sma_50', 'sma_200', 'rsi_14', 'macd_hist'],
  ['volatility_1y', 'max_drawdown_1y'],
]

export function formatMetric(name: string, value: number | undefined, currency: string, locale: string): string {
  switch (kinds[name] ?? 'number') {
    case 'fraction': return formatFraction(value, locale)
    case 'multiple': return value === undefined ? formatNumber(value, locale) : `${formatNumber(value, locale, 1)}x`
    case 'money': return formatMoney(value, currency, locale)
    case 'compactMoney': return formatCompact(value, currency, locale)
    default: return formatNumber(value, locale)
  }
}

/** Color for a 0-100 score, higher is better. */
export function scoreColor(value: number | undefined): 'success' | 'warning' | 'error' | 'neutral' {
  if (value === undefined) return 'neutral'
  if (value >= 65) return 'success'
  if (value >= 35) return 'warning'
  return 'error'
}
