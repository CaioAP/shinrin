// The key passes through here once, on its way to be sealed by the Go API.
// It is never logged, stored or sent back.
export default defineEventHandler(async (event) => {
  const { provider, model, baseUrl, apiKey, monthlyCap } = await readJSONBody(event)
  return backendFetch<LLMSettings>(event, '/api/v1/me/llm', { method: 'PUT', body: { provider, model, baseUrl, apiKey, monthlyCap } })
})
