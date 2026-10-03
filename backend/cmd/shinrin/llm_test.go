package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/port"
)

func TestPublicOnlyClientRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	_, err := publicOnlyClient(time.Second).Get(srv.URL)
	if err == nil || !strings.Contains(err.Error(), "non-public address") {
		t.Fatalf("loopback call err = %v", err)
	}
}

func TestConnectorUnknownProvider(t *testing.T) {
	if _, err := (llmConnector{}).Connect(port.LLMCredential{Provider: "gemini", APIKey: "k"}); err == nil {
		t.Fatal("unknown provider accepted")
	}
}
