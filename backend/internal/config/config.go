// Package config loads runtime settings from the environment. It is read once
// in cmd/ and passed down as plain values; nothing else reads env vars.
package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Config holds every setting the binary needs.
type Config struct {
	// HTTPAddr is where `shinrin api` listens (SHINRIN_ADDR).
	HTTPAddr string
	// ShutdownTimeout bounds graceful shutdown (SHINRIN_SHUTDOWN_TIMEOUT).
	ShutdownTimeout time.Duration
	// LogLevel is debug, info, warn or error (SHINRIN_LOG_LEVEL).
	LogLevel slog.Level
	// Version is reported by /api/v1/meta (SHINRIN_VERSION, set at deploy).
	Version string
}

// Load reads settings through getenv (os.Getenv in production, a map lookup in
// tests) and applies defaults.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		HTTPAddr:        or(getenv("SHINRIN_ADDR"), ":8080"),
		ShutdownTimeout: 10 * time.Second,
		LogLevel:        slog.LevelInfo,
		Version:         or(getenv("SHINRIN_VERSION"), "dev"),
	}

	if v := getenv("SHINRIN_SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("SHINRIN_SHUTDOWN_TIMEOUT: %w", err)
		}
		c.ShutdownTimeout = d
	}
	if v := getenv("SHINRIN_LOG_LEVEL"); v != "" {
		if err := c.LogLevel.UnmarshalText([]byte(strings.ToLower(v))); err != nil {
			return Config{}, fmt.Errorf("SHINRIN_LOG_LEVEL: %w", err)
		}
	}
	return c, nil
}

func or(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
