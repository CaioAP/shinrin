package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/anthropic"
	"github.com/CaioAP/shinrin/backend/internal/adapter/out/openai"
	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// llmTimeout bounds one report call; analyst notes are short but a model
// thinking hard can take a while.
const llmTimeout = 5 * time.Minute

// llmProviders is the registry of LLM adapters by provider name. Adding a
// vendor (Ollama in v2) is one entry here and one adapter package.
var llmProviders = map[string]func(port.LLMCredential) (port.LLMProvider, error){
	"anthropic": func(c port.LLMCredential) (port.LLMProvider, error) {
		return anthropic.New(c.APIKey, c.Model, c.BaseURL, &http.Client{Timeout: llmTimeout}), nil
	},
	"openai": func(c port.LLMCredential) (port.LLMProvider, error) {
		return openai.New(&http.Client{Timeout: llmTimeout}, c.BaseURL, c.APIKey, c.Model)
	},
}

// llmConnector implements port.LLMConnector over the registry.
type llmConnector struct{}

func (llmConnector) Connect(c port.LLMCredential) (port.LLMProvider, error) {
	f, ok := llmProviders[c.Provider]
	if !ok {
		return nil, fmt.Errorf("%w: unknown LLM provider %q (have anthropic, openai)", domain.ErrInvalid, c.Provider)
	}
	return f(c)
}
