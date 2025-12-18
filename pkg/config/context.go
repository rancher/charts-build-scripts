package config

import (
	"context"
	"errors"
)

type contextKey string

const configKey contextKey = "charts-build-scripts-config"

// WithConfig attaches a Config instance to the context and returns a new context
// Must be called during app init
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey, cfg)
}

// FromContext retrieves the Config instance from the context
// Returns nil if no config is attached to the context
// Must be called only after Config was initialized
func FromContext(ctx context.Context) (*Config, error) {
	cfg, _ := ctx.Value(configKey).(*Config)
	if cfg == nil {
		return nil, errors.New("config not initialized in context")
	}
	return cfg, nil
}

// SetSoftError will change SoftError Mode
func SetSoftError(ctx context.Context, newValue bool) {
	cfg, _ := FromContext(ctx)
	cfg.SoftErrorMode = newValue
}

// IsSoftError will return true if SoftError mode is enabled
func IsSoftError(ctx context.Context) bool {
	cfg, _ := FromContext(ctx)
	return cfg.SoftErrorMode
}
