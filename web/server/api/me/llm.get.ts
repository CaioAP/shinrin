export default defineEventHandler(event => backendFetch<LLMSettings>(event, '/api/v1/me/llm'))
