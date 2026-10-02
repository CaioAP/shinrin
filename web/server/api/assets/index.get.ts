export default defineEventHandler((event) => {
  const { market, class: assetClass, indexMember } = getQuery(event)
  return backendFetch<ListResponse<Asset>>(event, '/api/v1/assets', {
    // Forward only the filters the Go API understands; it validates them.
    query: {
      market: market?.toString(),
      class: assetClass?.toString(),
      indexMember: indexMember?.toString(),
    },
  })
})
