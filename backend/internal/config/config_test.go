package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/config"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := config.Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.ShutdownTimeout != 10*time.Second || c.LogLevel != slog.LevelInfo || c.Version != "dev" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadOverrides(t *testing.T) {
	c, err := config.Load(env(map[string]string{
		"SHINRIN_ADDR":             ":9000",
		"SHINRIN_SHUTDOWN_TIMEOUT": "3s",
		"SHINRIN_LOG_LEVEL":        "DEBUG",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":9000" || c.ShutdownTimeout != 3*time.Second || c.LogLevel != slog.LevelDebug {
		t.Fatalf("overrides not applied: %+v", c)
	}
}

func TestLoadRejectsBadDuration(t *testing.T) {
	if _, err := config.Load(env(map[string]string{"SHINRIN_SHUTDOWN_TIMEOUT": "soon"})); err == nil {
		t.Fatal("want error for bad duration")
	}
}

func TestLoadPipelineSettings(t *testing.T) {
	c, err := config.Load(env(map[string]string{
		"SHINRIN_DATABASE_URL":   "postgres://localhost/shinrin",
		"SHINRIN_HISTORY_START":  "2015-01-01",
		"SHINRIN_TIINGO_TOKEN":   "tok",
		"SHINRIN_SEC_USER_AGENT": "Shinrin me@example.com",
		"SHINRIN_FINNHUB_TOKEN":  "fh",
		"SHINRIN_BRAPI_TOKEN":    "br",
		"SHINRIN_FRED_API_KEY":   "fr",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.DatabaseURL == "" || c.HistoryStart.Year() != 2015 || c.TiingoToken != "tok" || c.SECUserAgent == "" ||
		c.FinnhubToken != "fh" || c.BrapiToken != "br" || c.FREDAPIKey != "fr" {
		t.Fatalf("pipeline settings not applied: %+v", c)
	}
	if _, err := config.Load(env(map[string]string{"SHINRIN_HISTORY_START": "2015"})); err == nil {
		t.Fatal("want error for bad date")
	}
}
