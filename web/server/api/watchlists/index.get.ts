export default defineEventHandler(event => backendFetch<ListResponse<Watchlist>>(event, '/api/v1/watchlists'))
