// Package system implements port.SystemService: service info and health.
package system

import (
	"context"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Service reports what is running and whether its dependencies are healthy.
type Service struct {
	version  string
	checkers []port.HealthChecker
}

var _ port.SystemService = (*Service)(nil)

// New builds the service. Every checker is run on each Health call.
func New(version string, checkers ...port.HealthChecker) *Service {
	return &Service{version: version, checkers: checkers}
}

// Info returns the service name, version and the mandatory disclaimer.
func (s *Service) Info(context.Context) port.SystemInfo {
	return port.SystemInfo{Name: "shinrin", Version: s.version, Disclaimer: domain.Disclaimer}
}

// Health runs every checker and reports OK only if all of them pass.
func (s *Service) Health(ctx context.Context) port.HealthReport {
	r := port.HealthReport{OK: true, Checks: make(map[string]string, len(s.checkers))}
	for _, c := range s.checkers {
		if err := c.Check(ctx); err != nil {
			r.OK = false
			r.Checks[c.Name()] = err.Error()
			continue
		}
		r.Checks[c.Name()] = "ok"
	}
	return r
}
