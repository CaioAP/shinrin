// Writing a report calls the user's LLM provider and can take a minute or two.
export default defineEventHandler(async (event) => {
  const market = encodeURIComponent(getRouterParam(event, 'market') ?? '')
  const symbol = encodeURIComponent(getRouterParam(event, 'symbol') ?? '')
  const { profile, lang } = await readJSONBody(event)
  setResponseStatus(event, 201)
  return backendFetch<AIReport>(event, `/api/v1/assets/${market}/${symbol}/reports`, { method: 'POST', body: { profile, lang } })
})
