// Package httpapi exposes the REST API consumed by the Nuxt app.
package httpapi

import (
	"encoding/json"
	"net/http"
)

// Disclaimer is returned by the API so every client can show it.
const Disclaimer = "Shinrin is an educational analysis tool. Nothing it shows is financial advice. It never places orders or connects to brokerages."

// NewRouter builds the API routes.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/meta", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"name":       "shinrin",
			"disclaimer": Disclaimer,
		})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
