package httpapi

import (
	"net/http"

	"github.com/CaioAP/shinrin/backend/internal/port"
)

type systemHandler struct {
	svc port.SystemService
}

func (h systemHandler) health(w http.ResponseWriter, r *http.Request) {
	rep := h.svc.Health(r.Context())
	if !rep.OK {
		writeJSON(w, http.StatusServiceUnavailable, healthDTO{Status: "degraded", Checks: rep.Checks})
		return
	}
	writeJSON(w, http.StatusOK, healthDTO{Status: "ok", Checks: rep.Checks})
}

func (h systemHandler) meta(w http.ResponseWriter, r *http.Request) {
	info := h.svc.Info(r.Context())
	writeJSON(w, http.StatusOK, metaDTO{Name: info.Name, Version: info.Version, Disclaimer: info.Disclaimer})
}
