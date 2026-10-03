// Package anthropic implements port.LLMProvider with the official Anthropic
// Go SDK (Messages API). Each client is bound to one user's own API key.
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultModel is used when the user picks none.
const DefaultModel = "claude-opus-5-5"

// fallbackModels accept the server-side "default" fallback, which re-runs a
// request declined by a safety classifier on Anthropic's recommended model
// instead of returning the refusal.
var fallbackModels = map[string]bool{
	"claude-fable-5-1": true, "claude-opus-5-5": true, "claude-opus-5": true, "claude-sonnet-5-5": true,
}

// Client calls the Messages API.
type Client struct {
	c        sdk.Client
	model    string
	fallback bool
}

var _ port.LLMProvider = (*Client)(nil)

// New binds a client to apiKey. baseURL overrides the endpoint (tests, a
// proxy) and disables the server-side fallback, which only the first-party
// API offers. httpClient may be nil.
func New(apiKey, model, baseURL string, httpClient *http.Client) *Client {
	opts := []option.RequestOption{option.WithAPIKey(apiKey), option.WithMaxRetries(2)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}
	if model == "" {
		model = DefaultModel
	}
	return &Client{c: sdk.NewClient(opts...), model: model, fallback: baseURL == "" && fallbackModels[model]}
}

// Name implements port.LLMProvider.
func (c *Client) Name() string { return "anthropic" }

// ErrRefused is returned when the model declines the request.
var ErrRefused = errors.New("anthropic: the model declined the request")

// Generate implements port.LLMProvider. A JSONSchema is sent as structured
// output, so the reply is guaranteed to parse against it.
func (c *Client) Generate(ctx context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 16000
	}
	params := sdk.BetaMessageNewParams{
		Model:     sdk.Model(model),
		MaxTokens: maxTokens,
		Messages:  []sdk.BetaMessageParam{sdk.NewBetaUserMessage(sdk.NewBetaTextBlock(req.Prompt))},
	}
	if req.System != "" {
		params.System = []sdk.BetaTextBlockParam{{Text: req.System}}
	}
	if len(req.JSONSchema) > 0 {
		params.OutputConfig = sdk.BetaOutputConfigParam{Format: sdk.BetaJSONOutputFormatParam{Schema: json.RawMessage(req.JSONSchema)}}
	}
	if c.fallback && (req.Model == "" || fallbackModels[req.Model]) {
		params.Betas = []sdk.AnthropicBeta{sdk.AnthropicBetaServerSideFallback2026_07_01}
		params.Fallbacks = sdk.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()}
	}

	msg, err := c.c.Beta.Messages.New(ctx, params)
	if err != nil {
		var apiErr *sdk.Error
		if errors.As(err, &apiErr) {
			return port.LLMResponse{}, fmt.Errorf("anthropic: HTTP %d", apiErr.StatusCode)
		}
		return port.LLMResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	if msg.StopReason == sdk.BetaStopReasonRefusal {
		return port.LLMResponse{}, ErrRefused
	}
	var text strings.Builder
	for _, b := range msg.Content {
		if t, ok := b.AsAny().(sdk.BetaTextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	if msg.StopReason == sdk.BetaStopReasonMaxTokens {
		return port.LLMResponse{}, fmt.Errorf("anthropic: reply cut off at %d tokens", maxTokens)
	}
	return port.LLMResponse{
		Text:      text.String(),
		Model:     string(msg.Model),
		TokensIn:  int(msg.Usage.InputTokens),
		TokensOut: int(msg.Usage.OutputTokens),
	}, nil
}
