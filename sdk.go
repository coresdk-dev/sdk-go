// Package coresdk provides the Go SDK for CoreSDK.
// Initialize with SDK.FromEnv() — reads CORESDK_* environment variables.
package coresdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// SDK is the main entry point. Initialize once per process.
type SDK struct {
	Config *Config
	client *Client
}

// FromEnv initializes the SDK from environment variables.
// All env vars have safe defaults — no required configuration.
func FromEnv() (*SDK, error) {
	cfg := ConfigFromEnv()
	client, err := NewClient(cfg)
	if err != nil {
		if cfg.FailMode == "open" {
			slog.Warn("CoreSDK sidecar unreachable — failing open", "error", err)
			client = nil
		} else {
			return nil, err
		}
	}
	return &SDK{Config: cfg, client: client}, nil
}

// Authorize validates a JWT and returns claims.
// Fails open (returns unknown claims, no error) when FailMode == "open" and sidecar is unreachable.
// Fails closed (returns error) when FailMode == "closed" and sidecar is unreachable.
func (s *SDK) Authorize(ctx context.Context, token string) (*Claims, error) {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: authorize fail-open (no client)")
		return &Claims{Subject: "unknown", FailOpen: true}, nil
	}
	claims, err := s.client.ValidateToken(ctx, token)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: authorize failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: authorize fail-open", "error", err)
		return &Claims{Subject: "unknown", FailOpen: true}, nil
	}
	return claims, nil
}

// EvaluatePolicy evaluates a Rego rule.
// Fails open (returns true) when FailMode == "open" and sidecar is unreachable.
// Fails closed (returns error) when FailMode == "closed".
func (s *SDK) EvaluatePolicy(ctx context.Context, rule string, input map[string]any) (bool, error) {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return false, fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: evaluate_policy fail-open (no client)")
		return true, nil
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return false, fmt.Errorf("coresdk: marshal input: %w", err)
		}
		return true, nil
	}
	result, err := s.client.EvaluatePolicy(ctx, rule, string(inputBytes))
	if err != nil {
		if s.Config.FailMode == "closed" {
			return false, fmt.Errorf("coresdk: policy eval failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: evaluate_policy fail-open", "error", err)
		return true, nil
	}
	return result, nil
}

// IsEnabled checks whether a feature flag is enabled by querying the control plane.
// Returns true (fail-open) if no control plane is configured or on error with FailMode "open".
func (s *SDK) IsEnabled(ctx context.Context, flagKey string) (bool, error) {
	if s.Config.ControlPlaneURL == "" {
		return true, nil
	}
	url := s.Config.ControlPlaneURL + "/api/v1/flags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return s.flagFailOpen(err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return s.flagFailOpen(err)
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return s.flagFailOpen(err)
	}
	flags, _ := result["flags"].([]any)
	for _, f := range flags {
		fm, _ := f.(map[string]any)
		if fm["key"] == flagKey || fm["name"] == flagKey {
			if enabled, ok := fm["enabled"].(bool); ok {
				return enabled, nil
			}
		}
	}
	return true, nil // unknown flag -> fail-open
}

func (s *SDK) flagFailOpen(err error) (bool, error) {
	if s.Config.FailMode == "closed" {
		return false, err
	}
	return true, nil
}

// Claims holds validated JWT claims.
type Claims struct {
	Subject  string
	TenantID string
	Roles    []string
	FailOpen bool
}
