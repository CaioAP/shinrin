export default defineEventHandler((event) => {
  const id = encodeURIComponent(getRouterParam(event, 'id') ?? '')
  return backendFetch<AIReport>(event, `/api/v1/reports/${id}`)
})
