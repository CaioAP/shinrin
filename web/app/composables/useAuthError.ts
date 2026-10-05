/** Turns a failed sign-in or sign-up into a translated message. */
export function useAuthError() {
  const { t } = useI18n()
  return (err: unknown): string => {
    const status = (err as { statusCode?: number }).statusCode
    const message = errorMessage(err)
    switch (status) {
      case 401: return t('auth.errors.unauthorized')
      case 409: return t('auth.errors.conflict')
      case 429: return t('auth.errors.rateLimited')
      case 400: return t('auth.errors.invalid', { message })
      default: return t('auth.errors.generic', { message })
    }
  }
}
