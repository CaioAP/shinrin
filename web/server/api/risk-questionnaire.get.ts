export default defineEventHandler(event => backendFetch<ListResponse<Question>>(event, '/api/v1/risk-questionnaire'))
