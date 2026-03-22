// Package coresdk provides the Go SDK for CoreSDK.
// Initialize with SDK.FromEnv() — reads CORESDK_* environment variables.
package coresdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

// IsEnabled checks whether a feature flag is enabled via the sidecar gRPC service.
// Delegates to EvaluateFlag for the actual gRPC call.
// Returns true (fail-open) when FailMode == "open" and sidecar is unreachable.
func (s *SDK) IsEnabled(ctx context.Context, flagKey string) (bool, error) {
	result, err := s.EvaluateFlag(ctx, flagKey, "")
	if err != nil {
		return false, err
	}
	return result.Enabled, nil
}

// RevokeToken revokes a JWT token via the sidecar.
func (s *SDK) RevokeToken(ctx context.Context, token string) error {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: revoke_token fail-open (no client)")
		return nil
	}
	err := s.client.RevokeToken(ctx, token)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return fmt.Errorf("coresdk: revoke_token failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: revoke_token fail-open", "error", err)
		return nil
	}
	return nil
}

// IsRevoked checks whether a token has been revoked.
func (s *SDK) IsRevoked(ctx context.Context, token string) (bool, error) {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return false, fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: is_revoked fail-open (no client)")
		return false, nil
	}
	revoked, err := s.client.IsRevoked(ctx, token)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return false, fmt.Errorf("coresdk: is_revoked failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: is_revoked fail-open", "error", err)
		return false, nil
	}
	return revoked, nil
}

// CheckRateLimit checks a rate limit key via the sidecar.
func (s *SDK) CheckRateLimit(ctx context.Context, key string) (*RateLimitDecision, error) {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: check_rate_limit fail-open (no client)")
		return &RateLimitDecision{Allowed: true}, nil
	}
	result, err := s.client.CheckRateLimit(ctx, key)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: check_rate_limit failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: check_rate_limit fail-open", "error", err)
		return &RateLimitDecision{Allowed: true}, nil
	}
	return result, nil
}

// EmitAuditEvent emits an audit event via the sidecar.
func (s *SDK) EmitAuditEvent(ctx context.Context, action, userID, outcome string, metadata map[string]string) error {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: emit_audit_event fail-open (no client)")
		return nil
	}
	err := s.client.EmitAuditEvent(ctx, action, userID, outcome, metadata)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return fmt.Errorf("coresdk: emit_audit_event failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: emit_audit_event fail-open", "error", err)
		return nil
	}
	return nil
}

// EvaluateFlag evaluates a feature flag via the sidecar.
func (s *SDK) EvaluateFlag(ctx context.Context, key, userID string) (*FlagDecision, error) {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: evaluate_flag fail-open (no client)")
		return &FlagDecision{Enabled: true, Key: key}, nil
	}
	result, err := s.client.EvaluateFlag(ctx, key, userID)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: evaluate_flag failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: evaluate_flag fail-open", "error", err)
		return &FlagDecision{Enabled: true, Key: key}, nil
	}
	return result, nil
}

// CheckEntitlement checks a license entitlement via the sidecar.
func (s *SDK) CheckEntitlement(ctx context.Context, key string) (*LicenseInfo, error) {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: check_entitlement fail-open (no client)")
		return &LicenseInfo{Allowed: true}, nil
	}
	result, err := s.client.CheckEntitlement(ctx, key)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: check_entitlement failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: check_entitlement fail-open", "error", err)
		return &LicenseInfo{Allowed: true}, nil
	}
	return result, nil
}

// RateLimitDecision holds the result of a rate limit check.
type RateLimitDecision struct {
	Allowed   bool
	Remaining int64
	ResetAt   int64
}

// FlagDecision holds the result of a feature flag evaluation.
type FlagDecision struct {
	Enabled bool
	Key     string
}

// LicenseInfo holds the result of a license entitlement check.
type LicenseInfo struct {
	Allowed  bool
	Plan     string
	Features []string
}

// Claims holds validated JWT claims.
type Claims struct {
	Subject  string
	TenantID string
	Roles    []string
	FailOpen bool
}
