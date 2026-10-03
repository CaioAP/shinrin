export default defineEventHandler((event) => {
  const { market, class: assetClass, profile, limit } = getQuery(event)
  return backendFetch<Ranking>(event, '/api/v1/rankings', {
    // Forward only the filters the Go API understands; it validates them.
    query: {
      market: market?.toString(),
      class: assetClass?.toString(),
      profile: profile?.toString(),
      limit: limit?.toString(),
    },
  })
})
