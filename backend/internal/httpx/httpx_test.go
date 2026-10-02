package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/httpx"
)

func TestRetryAndUserAgent(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "shinrin test@example.com" {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpx.NewClient(httpx.Options{UserAgent: "shinrin test@example.com", Retries: 3, RetryBase: time.Millisecond})
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || calls.Load() != 3 {
		t.Fatalf("status %d after %d calls", resp.StatusCode, calls.Load())
	}
}

func TestNoRetryOnClientError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := httpx.NewClient(httpx.Options{Retries: 3, RetryBase: time.Millisecond})
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

func TestRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	c := httpx.NewClient(httpx.Options{RequestsPerSecond: 20, Burst: 1})
	start := time.Now()
	for range 3 {
		resp, err := c.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if el := time.Since(start); el < 90*time.Millisecond {
		t.Fatalf("3 requests at 20/s took %v, want >= 100ms", el)
	}
}
