// Package httpx builds the outbound HTTP clients that data-provider adapters
// use. Politeness rules (a User-Agent naming the app, a request rate under
// the provider's free quota, retries with backoff) are the same for every
// provider, so they are written once here as http.RoundTripper decorators and
// configured per provider in cmd/shinrin/wire.go. Adapters just receive an
// *http.Client.
package httpx

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

// Options configure one provider's client. Zero values disable a feature.
type Options struct {
	// UserAgent is sent on every request. SEC EDGAR rejects requests
	// without a descriptive one ("app-name contact@example.com").
	UserAgent string
	// RequestsPerSecond and Burst size the token bucket. Set them below the
	// provider's free quota.
	RequestsPerSecond float64
	Burst             int
	// Retries is how many times a GET is retried on 429, 5xx or a network
	// error, with exponential backoff starting at RetryBase.
	Retries   int
	RetryBase time.Duration
	// Timeout bounds a whole request including retries' reads of the body.
	Timeout time.Duration
	// Base is the transport to decorate. Default: http.DefaultTransport.
	Base http.RoundTripper
}

// NewClient returns a client whose transport is Base wrapped, innermost first,
// in retry, rate limit and user agent decorators. Rate limiting sits outside
// retry, so a retried request waits for a token like any other.
func NewClient(o Options) *http.Client {
	rt := o.Base
	if rt == nil {
		rt = http.DefaultTransport
	}
	if o.Retries > 0 {
		base := o.RetryBase
		if base <= 0 {
			base = 500 * time.Millisecond
		}
		rt = &retry{next: rt, max: o.Retries, base: base}
	}
	if o.RequestsPerSecond > 0 {
		rt = &limit{next: rt, lim: rate.NewLimiter(rate.Limit(o.RequestsPerSecond), max(1, o.Burst))}
	}
	if o.UserAgent != "" {
		rt = &userAgent{next: rt, ua: o.UserAgent}
	}
	return &http.Client{Transport: rt, Timeout: o.Timeout}
}

type userAgent struct {
	next http.RoundTripper
	ua   string
}

func (t *userAgent) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("User-Agent", t.ua)
	return t.next.RoundTrip(r)
}

type limit struct {
	next http.RoundTripper
	lim  *rate.Limiter
}

func (t *limit) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.lim.Wait(r.Context()); err != nil {
		return nil, err
	}
	return t.next.RoundTrip(r)
}

type retry struct {
	next http.RoundTripper
	max  int
	base time.Duration
}

func (t *retry) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return t.next.RoundTrip(r)
	}
	for attempt := 0; ; attempt++ {
		resp, err := t.next.RoundTrip(r)
		if attempt >= t.max || !retryable(resp, err) || r.Context().Err() != nil {
			return resp, err
		}
		wait := t.base << attempt
		if resp != nil {
			if ra := retryAfter(resp); ra > 0 {
				wait = ra
			}
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
		}
		if err := sleep(r.Context(), wait); err != nil {
			return nil, err
		}
	}
}

func retryable(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
}

func retryAfter(resp *http.Response) time.Duration {
	s := resp.Header.Get("Retry-After")
	if s == "" {
		return 0
	}
	if secs, err := strconv.Atoi(s); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(s); err == nil {
		return time.Until(t)
	}
	return 0
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
