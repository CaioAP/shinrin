package main

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
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
var llmProviders = map[string]func(port.LLMCredential, *http.Client) (port.LLMProvider, error){
	"anthropic": func(c port.LLMCredential, hc *http.Client) (port.LLMProvider, error) {
		return anthropic.New(c.APIKey, c.Model, c.BaseURL, hc), nil
	},
	"openai": func(c port.LLMCredential, hc *http.Client) (port.LLMProvider, error) {
		return openai.New(hc, c.BaseURL, c.APIKey, c.Model)
	},
}

// llmConnector implements port.LLMConnector over the registry. The CLI
// runs with the operator's own settings; the web app's connector is
// publicOnly, because users choose the base URL.
type llmConnector struct {
	publicOnly bool
}

func (l llmConnector) Connect(c port.LLMCredential) (port.LLMProvider, error) {
	f, ok := llmProviders[c.Provider]
	if !ok {
		return nil, fmt.Errorf("%w: unknown LLM provider %q (have anthropic, openai)", domain.ErrInvalid, c.Provider)
	}
	hc := &http.Client{Timeout: llmTimeout}
	if l.publicOnly {
		hc = publicOnlyClient(llmTimeout)
	}
	return f(c, hc)
}

// publicOnlyClient refuses to connect to loopback, private, link-local or
// metadata addresses, checked on the resolved IP at dial time so DNS
// tricks cannot point a user's base URL inside the server's network. It
// skips HTTP proxies on purpose: through a proxy the dial check would see
// the proxy, not the destination.
func publicOnlyClient(timeout time.Duration) *http.Client {
	d := &net.Dialer{Timeout: 10 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		if ip := net.ParseIP(host); ip == nil || !domain.PublicIP(ip) {
			return fmt.Errorf("%w: refusing to call non-public address %s", domain.ErrInvalid, host)
		}
		return nil
	}}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	t.DialContext = d.DialContext
	return &http.Client{Timeout: timeout, Transport: t}
}
