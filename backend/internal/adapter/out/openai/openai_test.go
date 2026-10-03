package openai_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/openai"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

func TestGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer sk-or-test" {
			t.Errorf("request %s auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		rf := body["response_format"].(map[string]any)
		js := rf["json_schema"].(map[string]any)
		msgs := body["messages"].([]any)
		if rf["type"] != "json_schema" || js["strict"] != true || body["model"] != "openai/gpt-x" || len(msgs) != 2 {
			t.Errorf("body = %v", body)
		}
		_, _ = io.WriteString(w, `{"model":"openai/gpt-x","choices":[{"message":{"content":"{\"summary\":\"ok\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":90,"completion_tokens":30}}`)
	}))
	defer srv.Close()
	c, err := openai.New(nil, srv.URL+"/v1/", "sk-or-test", "openai/gpt-x")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Generate(context.Background(), port.LLMRequest{System: "s", Prompt: "p", JSONSchema: []byte(`{"type":"object"}`)})
	if err != nil || resp.Text != `{"summary":"ok"}` || resp.TokensIn != 90 || resp.TokensOut != 30 {
		t.Errorf("Generate = %+v, %v", resp, err)
	}
}

func TestErrors(t *testing.T) {
	if _, err := openai.New(nil, "", "k", ""); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("no model err = %v", err)
	}
	for name, tc := range map[string]struct {
		status int
		body   string
		want   error
	}{
		"refusal": {200, `{"choices":[{"message":{"refusal":"no"},"finish_reason":"stop"}]}`, openai.ErrRefused},
		"401":     {401, `{"error":{"message":"bad key"}}`, nil},
		"cut off": {200, `{"choices":[{"message":{"content":"{"},"finish_reason":"length"}]}`, nil},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}))
		c, _ := openai.New(nil, srv.URL, "k", "m")
		_, err := c.Generate(context.Background(), port.LLMRequest{Prompt: "p"})
		if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Errorf("%s: err = %v", name, err)
		}
		srv.Close()
	}
}
