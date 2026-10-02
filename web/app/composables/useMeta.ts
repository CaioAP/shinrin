/** Service name, version and the disclaimer text, fetched once per page load. */
export function useMeta() {
  return useFetch('/api/meta', { key: 'meta' })
}
