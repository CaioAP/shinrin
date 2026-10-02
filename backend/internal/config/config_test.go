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
