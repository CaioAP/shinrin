package anthropic_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/anthropic"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// redirect sends every request to the test server, so the client runs with
// its production base URL (and therefore its first-party-only options).
type redirect struct{ to *url.URL }

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme, req.URL.Host = r.to.Scheme, r.to.Host
	return http.DefaultTransport.RoundTrip(req)
}

func server(t *testing.T, reply string, check func(*http.Request, map[string]any)) *http.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("request body: %v", err)
		}
		check(r, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return &http.Client{Transport: redirect{u}}
}

const okReply = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5",
"content":[{"type":"text","text":"{\"summary\":\"ok\"}"}],"stop_reason":"end_turn","usage":{"input_tokens":120,"output_tokens":40}}`

func TestGenerate(t *testing.T) {
	hc := server(t, okReply, func(r *http.Request, body map[string]any) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != "sk-test" {
			t.Errorf("request %s, key %q", r.URL.Path, r.Header.Get("X-Api-Key"))
		}
		if !strings.Contains(r.Header.Get("Anthropic-Beta"), "server-side-fallback-2026-07-01") || body["fallbacks"] != "default" {
			t.Errorf("fallback not requested: %v %v", r.Header.Get("Anthropic-Beta"), body["fallbacks"])
		}
		format := body["output_config"].(map[string]any)["format"].(map[string]any)
		if format["type"] != "json_schema" || format["schema"].(map[string]any)["type"] != "object" {
			t.Errorf("format = %v", format)
		}
		if body["model"] != "claude-opus-5-5" || body["system"].([]any)[0].(map[string]any)["text"] != "be an analyst" {
			t.Errorf("body = %v", body)
		}
	})
	c := anthropic.New("sk-test", "", "", hc)
	resp, err := c.Generate(context.Background(), port.LLMRequest{System: "be an analyst", Prompt: "data", JSONSchema: []byte(`{"type":"object"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != `{"summary":"ok"}` || resp.TokensIn != 120 || resp.TokensOut != 40 || resp.Model != "claude-opus-5-5" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestGenerateNoFallbackForOtherModels(t *testing.T) {
	hc := server(t, okReply, func(r *http.Request, body map[string]any) {
		if _, ok := body["fallbacks"]; ok || r.Header.Get("Anthropic-Beta") != "" {
			t.Errorf("haiku must not ask for a fallback: %v", body)
		}
	})
	if _, err := anthropic.New("sk-test", "claude-haiku-4-5", "", hc).Generate(context.Background(), port.LLMRequest{Prompt: "x"}); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateRefusalAndErrors(t *testing.T) {
	refusal := `{"id":"msg_2","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":"refusal","usage":{"input_tokens":1,"output_tokens":0}}`
	c := anthropic.New("k", "", "", server(t, refusal, func(*http.Request, map[string]any) {}))
	if _, err := c.Generate(context.Background(), port.LLMRequest{Prompt: "x"}); !errors.Is(err, anthropic.ErrRefused) {
		t.Errorf("refusal err = %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	}))
	defer srv.Close()
	_, err := anthropic.New("sk-wrong-key", "", srv.URL, nil).Generate(context.Background(), port.LLMRequest{Prompt: "x"})
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "sk-wrong") {
		t.Errorf("401 err = %v", err)
	}
}
