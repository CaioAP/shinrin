package domain_test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

func TestValidateLLMEndpoint(t *testing.T) {
	ok := map[string]string{
		"":                                "",
		"https://openrouter.ai/api/v1/":   "https://openrouter.ai/api/v1",
		" https://api.groq.com/openai/v1": "https://api.groq.com/openai/v1",
		"https://8.8.8.8/v1":              "https://8.8.8.8/v1",
	}
	for in, want := range ok {
		got, err := domain.ValidateLLMEndpoint(in)
		if err != nil || got != want {
			t.Errorf("ValidateLLMEndpoint(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{
		"http://api.openai.com/v1", "https://localhost/v1", "https://127.0.0.1/v1", "https://10.0.0.5/v1",
		"https://169.254.169.254/latest", "https://[::1]/v1", "https://metadata.google.internal/", "https://user:pw@api.x.com/",
		"https://api.x.com:8443/", "https://api.x.com/?k=1", "ftp://x.com", "not a url", "https://intranet/v1", "https://100.64.1.1/",
	} {
		if _, err := domain.ValidateLLMEndpoint(in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("ValidateLLMEndpoint(%q) err = %v, want ErrInvalid", in, err)
		}
	}
}

func TestPublicIP(t *testing.T) {
	for ip, want := range map[string]bool{"1.1.1.1": true, "192.168.0.1": false, "fd00::1": false, "2606:4700::1111": true, "0.0.0.0": false} {
		if got := domain.PublicIP(net.ParseIP(ip)); got != want {
			t.Errorf("PublicIP(%s) = %v", ip, got)
		}
	}
}

func TestNormalizeMonthlyCap(t *testing.T) {
	if n, err := domain.NormalizeMonthlyCap(0); n != domain.DefaultMonthlyReportCap || err != nil {
		t.Fatalf("default = %d, %v", n, err)
	}
	for _, n := range []int{-1, domain.MaxMonthlyReportCap + 1} {
		if _, err := domain.NormalizeMonthlyCap(n); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("NormalizeMonthlyCap(%d) err = %v", n, err)
		}
	}
	if got := domain.MonthStart(time.Date(2026, 10, 3, 18, 0, 0, 0, time.FixedZone("BRT", -3*3600))); !got.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("MonthStart = %v", got)
	}
}
