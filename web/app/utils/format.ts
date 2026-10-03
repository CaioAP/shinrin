// Pure formatting helpers, auto-imported. They take the locale explicitly so
// they stay testable and free of Vue state.

const missing = '—'

export function formatMoney(value: number | undefined, currency: string, locale: string): string {
  if (value === undefined || !Number.isFinite(value)) return missing
  return new Intl.NumberFormat(locale, { style: 'currency', currency, maximumFractionDigits: 2 }).format(value)
}

/** Formats a fraction (0.05) as a percentage (5.0%). */
export function formatFraction(value: number | undefined, locale: string, digits = 1): string {
  if (value === undefined || !Number.isFinite(value)) return missing
  return new Intl.NumberFormat(locale, { style: 'percent', minimumFractionDigits: digits, maximumFractionDigits: digits }).format(value)
}

/** Formats a value already in percent (15 for 15%). */
export function formatPercent(value: number | undefined, locale: string, digits = 2): string {
  if (value === undefined || !Number.isFinite(value)) return missing
  return formatFraction(value / 100, locale, digits)
}

export function formatNumber(value: number | undefined, locale: string, digits = 2): string {
  if (value === undefined || !Number.isFinite(value)) return missing
  return new Intl.NumberFormat(locale, { maximumFractionDigits: digits }).format(value)
}

/** Large amounts (market cap) in compact form: R$ 1,2 bi, $350M. */
export function formatCompact(value: number | undefined, currency: string, locale: string): string {
  if (value === undefined || !Number.isFinite(value)) return missing
  return new Intl.NumberFormat(locale, { style: 'currency', currency, notation: 'compact', maximumFractionDigits: 1 }).format(value)
}

/** Formats an ISO date (2026-10-02) or timestamp without shifting the day. */
export function formatDate(iso: string | undefined, locale: string): string {
  if (!iso) return missing
  const dateOnly = iso.length === 10
  const date = new Date(dateOnly ? `${iso}T12:00:00Z` : iso)
  if (Number.isNaN(date.getTime())) return missing
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeZone: dateOnly ? 'UTC' : undefined }).format(date)
}
