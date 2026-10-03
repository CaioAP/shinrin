package domain

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Monthly AI report caps. The cap is the user's own spending guard: reports
// run on their key, so the server only enforces what they chose.
const (
	DefaultMonthlyReportCap = 30
	MaxMonthlyReportCap     = 500
)

// LLMSettings describe a user's saved LLM account. The key itself never
// leaves the credential store; only its last four characters are shown.
type LLMSettings struct {
	Provider   string
	Model      string // empty means the provider's default
	BaseURL    string // empty means the provider's own endpoint
	KeyHint    string
	MonthlyCap int
	UpdatedAt  time.Time
}

// LLMUsage is how many reports a user generated this calendar month (UTC)
// against their cap.
type LLMUsage struct {
	Used  int
	Cap   int
	Since time.Time
}

// MonthStart is the first instant of t's calendar month in UTC, where the
// monthly cap resets.
func MonthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// NormalizeMonthlyCap applies the default to zero and rejects values out
// of range.
func NormalizeMonthlyCap(n int) (int, error) {
	switch {
	case n == 0:
		return DefaultMonthlyReportCap, nil
	case n < 1 || n > MaxMonthlyReportCap:
		return 0, fmt.Errorf("%w: the monthly cap must be between 1 and %d", ErrInvalid, MaxMonthlyReportCap)
	}
	return n, nil
}

// ValidateLLMEndpoint checks a user-supplied provider URL. The server calls
// it with the user's key, so it must be public HTTPS: no plain HTTP, no
// credentials in the URL, no localhost or private address literals. Names
// that resolve to private addresses are refused again when dialing.
func ValidateLLMEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	bad := func(why string) (string, error) {
		return "", fmt.Errorf("%w: base URL %s", ErrInvalid, why)
	}
	if len(raw) > 300 {
		return bad("is too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return bad("is not a URL")
	}
	if u.Scheme != "https" {
		return bad("must use https")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return bad("must not carry credentials, a query or a fragment")
	}
	if u.Port() != "" && u.Port() != "443" {
		return bad("must use the standard https port")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return bad("must be a public host")
	}
	if ip := net.ParseIP(host); ip != nil && !PublicIP(ip) {
		return bad("must be a public host")
	}
	if !strings.Contains(host, ".") && net.ParseIP(host) == nil {
		return bad("must be a public host")
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}

// PublicIP reports whether ip is a globally routable unicast address.
func PublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() &&
		!cgnat.Contains(ip) && !ip.Equal(net.IPv4(169, 254, 169, 254))
}

// cgnat is the shared address space (RFC 6598), private in practice.
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}
