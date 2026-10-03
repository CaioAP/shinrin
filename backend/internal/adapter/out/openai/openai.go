// Package openai implements port.LLMProvider for any API that speaks the
// OpenAI Chat Completions format (OpenAI itself, OpenRouter, and many
// self-hosted gateways). Each client is bound to one user's own API key.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultBaseURL is OpenAI's API.
const DefaultBaseURL = "https://api.openai.com/v1"

// Client calls POST {base}/chat/completions.
type Client struct {
	hc     *http.Client
	base   string
	apiKey string
	model  string
}

var _ port.LLMProvider = (*Client)(nil)

// New binds a client to apiKey. The model is required: names differ per
// gateway, so there is no safe default. baseURL may be empty for OpenAI.
func New(hc *http.Client, baseURL, apiKey, model string) (*Client, error) {
	if model == "" {
		return nil, fmt.Errorf("%w: an OpenAI-compatible provider needs a model name", domain.ErrInvalid)
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{hc: hc, base: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model}, nil
}

// Name implements port.LLMProvider.
func (c *Client) Name() string { return "openai" }

// ErrRefused is returned when the model declines the request.
var ErrRefused = errors.New("openai: the model declined the request")

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	MaxTokens      int             `json:"max_completion_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type       string `json:"type"`
	JSONSchema struct {
		Name   string          `json:"name"`
		Strict bool            `json:"strict"`
		Schema json.RawMessage `json:"schema"`
	} `json:"json_schema"`
}

type response struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Refusal string `json:"refusal"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Generate implements port.LLMProvider. A JSONSchema is sent as a strict
// json_schema response format.
func (c *Client) Generate(ctx context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	body := request{Model: model, MaxTokens: req.MaxTokens}
	if req.System != "" {
		body.Messages = append(body.Messages, message{"system", req.System})
	}
	body.Messages = append(body.Messages, message{"user", req.Prompt})
	if len(req.JSONSchema) > 0 {
		f := &responseFormat{Type: "json_schema"}
		f.JSONSchema.Name, f.JSONSchema.Strict, f.JSONSchema.Schema = "report", true, req.JSONSchema
		body.ResponseFormat = f
	}
	b, err := json.Marshal(body)
	if err != nil {
		return port.LLMResponse{}, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return port.LLMResponse{}, err
	}
	hreq.Header.Set("Authorization", "Bearer "+c.apiKey)
	hreq.Header.Set("Content-Type", "application/json")
	res, err := c.hc.Do(hreq)
	if err != nil {
		return port.LLMResponse{}, fmt.Errorf("openai: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return port.LLMResponse{}, fmt.Errorf("openai: read reply: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		// The error body is the provider's; it never contains our key.
		return port.LLMResponse{}, fmt.Errorf("openai: HTTP %d: %s", res.StatusCode, truncate(string(raw), 300))
	}
	var r response
	if err := json.Unmarshal(raw, &r); err != nil {
		return port.LLMResponse{}, fmt.Errorf("openai: decode reply: %w", err)
	}
	if len(r.Choices) == 0 {
		return port.LLMResponse{}, errors.New("openai: reply has no choices")
	}
	ch := r.Choices[0]
	if ch.Message.Refusal != "" {
		return port.LLMResponse{}, ErrRefused
	}
	if ch.FinishReason == "length" {
		return port.LLMResponse{}, errors.New("openai: reply cut off at the token limit")
	}
	return port.LLMResponse{Text: ch.Message.Content, Model: r.Model, TokensIn: r.Usage.PromptTokens, TokensOut: r.Usage.CompletionTokens}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
