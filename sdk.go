// Package coresdk provides the Go SDK for CoreSDK.
// Initialize with SDK.FromEnv() — reads CORESDK_* environment variables.
package coresdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
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

// AuthorizeOption configures a single Authorize call.
//
// Phase 1 of the app-store/scope/permission system introduces RequiredScope —
// see WithRequiredScope. The proto field `AuthorizeRequest.required_scope`
// (tag 8) is set on the wire when the sidecar's regenerated stubs are wired
// in; until then the option is parsed into authorizeOptions and threaded
// through any future Authorize-RPC code path without breaking existing
// callers.
type AuthorizeOption func(*authorizeOptions)

type authorizeOptions struct {
	// RequiredScope is an RFC 6749 §3.3 space-separated scope list. Multiple
	// values mean "all of these" (logical AND). A granted `jobs.*` satisfies
	// any required `jobs.<action>`.
	RequiredScope string
}

// WithRequiredScope sets an OAuth 2.0 scope filter for the Authorize call.
//
// The format is RFC 6749 §3.3: a single string, scopes separated by ASCII
// whitespace. Multiple scopes mean "all of these" (logical AND). On the
// sidecar side, a granted scope of `jobs.*` satisfies a required
// `jobs.write` thanks to dot-boundary wildcard matching.
func WithRequiredScope(scope string) AuthorizeOption {
	return func(o *authorizeOptions) { o.RequiredScope = scope }
}

// Authorize validates a JWT and returns claims.
// Fails open (returns unknown claims, no error) when FailMode == "open" and sidecar is unreachable.
// Fails closed (returns error) when FailMode == "closed" and sidecar is unreachable.
//
// Variadic AuthorizeOption values configure scope checks and other per-call
// behaviour. Existing callers without options continue to compile unchanged.
func (s *SDK) Authorize(ctx context.Context, token string, opts ...AuthorizeOption) (*Claims, error) {
	// Resolve options. Currently RequiredScope is parsed but not yet threaded
	// to the gRPC wire — that happens once the regenerated `AuthorizeRequest`
	// stub exposes RequiredScope (proto v1.2).
	var o authorizeOptions
	for _, opt := range opts {
		opt(&o)
	}
	_ = o // silence unused until wire is hooked up
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

// ExplainAuthorize validates a token and returns a structured explanation of the decision.
// Always returns an ExplainResult (never an error) — fail-open by design.
func (s *SDK) ExplainAuthorize(ctx context.Context, token string) (*ExplainResult, error) {
	claims, err := s.Authorize(ctx, token)
	if err != nil {
		return &ExplainResult{
			Outcome: "denied",
			Auth:    map[string]interface{}{"error": err.Error()},
		}, nil
	}
	subject := ""
	if claims != nil {
		subject = claims.Subject
	}
	return &ExplainResult{
		Outcome: "allowed",
		Auth:    map[string]interface{}{"subject": subject, "allowed": true},
	}, nil
}

// MintAgentToken mints a short-lived scoped JWT for agent-to-agent calls.
// ttl is clamped to [1, 300] seconds.
func (s *SDK) MintAgentToken(ctx context.Context, parentToken, targetService string, scopes []string, ttl time.Duration) (*AgentToken, error) {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: mint_agent_token fail-open (no client)")
		return &AgentToken{Token: "fail-open-agent-token", ExpiresInSeconds: 300, AgentChain: []string{targetService}}, nil
	}
	ttlSecs := int(ttl.Seconds())
	if ttlSecs <= 0 || ttlSecs > 300 {
		ttlSecs = 300
	}
	result, err := s.client.MintAgentToken(ctx, parentToken, targetService, scopes, ttlSecs)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: mint_agent_token failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: mint_agent_token fail-open", "error", err)
		return &AgentToken{Token: "fail-open-agent-token", ExpiresInSeconds: 300, AgentChain: []string{targetService}}, nil
	}
	return result, nil
}

// CheckEgress checks whether an outbound URL is safe (SSRF protection).
// Returns allowed=true if the sidecar is unreachable (fail-open).
func (s *SDK) CheckEgress(ctx context.Context, rawURL string) (*EgressDecision, error) {
	if s.client == nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: sidecar unreachable (fail-closed)")
		}
		slog.Warn("coresdk: check_egress fail-open (no client)")
		return &EgressDecision{Allowed: true}, nil
	}
	result, err := s.client.CheckEgress(ctx, rawURL)
	if err != nil {
		if s.Config.FailMode == "closed" {
			return nil, fmt.Errorf("coresdk: check_egress failed (fail-closed): %w", err)
		}
		slog.Warn("coresdk: check_egress fail-open", "error", err)
		return &EgressDecision{Allowed: true}, nil
	}
	return result, nil
}

// ExplainResult contains a structured explanation of an auth decision.
type ExplainResult struct {
	RequestID string                 `json:"request_id"`
	Outcome   string                 `json:"outcome"` // "allowed" | "denied"
	Auth      map[string]interface{} `json:"auth"`
	Policy    map[string]interface{} `json:"policy"`
	RateLimit map[string]interface{} `json:"rate_limit"`
	Masking   map[string]interface{} `json:"masking"`
	LatencyMs float64                `json:"latency_ms"`
}

// AgentToken is a short-lived scoped JWT for agent-to-agent delegation.
type AgentToken struct {
	Token            string   `json:"token"`
	ExpiresInSeconds int      `json:"expires_in_seconds"`
	AgentChain       []string `json:"agent_chain"`
}

// EgressDecision is the result of an outbound URL safety check.
type EgressDecision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
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
