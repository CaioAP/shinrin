export default defineEventHandler(event => backendFetch<Meta>(event, '/api/v1/meta'))
