package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

type marketHandler struct {
	svc port.MarketService
	now func() time.Time
}

// priceRanges maps the chart's range buttons to a look-back.
var priceRanges = map[string]func(time.Time) time.Time{
	"1m":  func(t time.Time) time.Time { return t.AddDate(0, -1, 0) },
	"3m":  func(t time.Time) time.Time { return t.AddDate(0, -3, 0) },
	"6m":  func(t time.Time) time.Time { return t.AddDate(0, -6, 0) },
	"1y":  func(t time.Time) time.Time { return t.AddDate(-1, 0, 0) },
	"5y":  func(t time.Time) time.Time { return t.AddDate(-5, 0, 0) },
	"max": func(time.Time) time.Time { return time.Time{} },
}

const (
	maxNewsLimit     = 50
	maxDividendYears = 20
)

// prices handles GET /api/v1/assets/{market}/{symbol}/prices?range=1y.
func (h marketHandler) prices(w http.ResponseWriter, r *http.Request) {
	key, err := parseAssetKey(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	rng := r.URL.Query().Get("range")
	if rng == "" {
		rng = "1y"
	}
	from, ok := priceRanges[rng]
	if !ok {
		writeDomainError(w, fmt.Errorf("%w: range must be one of 1m, 3m, 6m, 1y, 5y, max", domain.ErrInvalid))
		return
	}
	bars, err := h.svc.PriceHistory(r.Context(), key, from(h.now()))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := pricesDTO{Range: rng, Items: make([]priceBarDTO, len(bars))}
	for i, b := range bars {
		out.Items[i] = priceBarDTO{Date: dateString(b.Date), Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, AdjClose: b.AdjClose, Volume: b.Volume}
		out.Source = b.Source
	}
	writeJSON(w, http.StatusOK, out)
}

// dividends handles GET /api/v1/assets/{market}/{symbol}/dividends?years=5.
func (h marketHandler) dividends(w http.ResponseWriter, r *http.Request) {
	key, err := parseAssetKey(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	years, err := intParam(r, "years", 5, maxDividendYears)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	acts, err := h.svc.Dividends(r.Context(), key, h.now().AddDate(-years, 0, 0))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]dividendDTO, len(acts))
	for i, a := range acts {
		items[i] = dividendDTO{ExDate: dateString(a.ExDate), Type: string(a.Type), Value: a.Value, Source: a.Source}
	}
	writeJSON(w, http.StatusOK, listDTO[dividendDTO]{Items: items})
}

// news handles GET /api/v1/assets/{market}/{symbol}/news?limit=20.
func (h marketHandler) news(w http.ResponseWriter, r *http.Request) {
	key, err := parseAssetKey(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	limit, err := intParam(r, "limit", 20, maxNewsLimit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items, err := h.svc.News(r.Context(), key, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]newsDTO, len(items))
	for i, n := range items {
		out[i] = newsDTO{URL: n.URL, Title: n.Title, Summary: n.Summary, Lang: n.Lang, Source: n.Source, PublishedAt: n.PublishedAt.UTC().Format(time.RFC3339)}
	}
	writeJSON(w, http.StatusOK, listDTO[newsDTO]{Items: out})
}

// macro handles GET /api/v1/macro.
func (h marketHandler) macro(w http.ResponseWriter, r *http.Request) {
	m, err := h.svc.Macro(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := macroDTO{Items: make([]macroIndicatorDTO, len(m.Items)), Missing: nonNil(m.Missing)}
	for i, x := range m.Items {
		out.Items[i] = macroIndicatorDTO{Code: x.Code, Value: x.Value, Unit: x.Unit, AsOf: dateString(x.AsOf), Source: x.Source}
	}
	writeJSON(w, http.StatusOK, out)
}

// intParam reads an optional integer query parameter in 1..max.
func intParam(r *http.Request, name string, def, upper int) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > upper {
		return 0, fmt.Errorf("%w: %s must be 1 to %d", domain.ErrInvalid, name, upper)
	}
	return n, nil
}
