export default defineEventHandler(event => backendFetch<MacroStrip>(event, '/api/v1/macro'))
