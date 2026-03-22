package coresdk

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// MockConfig configures MockSDK behavior for tests.
// Zero value = allow all (safe default for tests).
type MockConfig struct {
	// DenyAll makes Authorize return an error for every call.
	// Zero value (false) means allow — correct default for most tests.
	DenyAll bool
	Claims  *Claims
}

// MockSpan records a single OTel-like span for assertion in tests.
type MockSpan struct {
	Name       string
	Attributes map[string]string
}

// AuthDecision holds the result of an authorization check.
type AuthDecision struct {
	Allowed bool
	Claims  *Claims
}

// MockSDK is a test double for SDK. Tracks calls for assertion.
type MockSDK struct {
	Config              *Config
	cfg                 MockConfig
	AllowAll            bool
	Flags               map[string]bool
	Spans               []MockSpan
	AuthorizeCalls      []string
	PolicyCalls         []string
	RevokeTokenCalls    []string
	IsRevokedCalls      []string
	RateLimitCalls      []string
	AuditCalls          []string
	FlagCalls           []string
	EntitlementCalls    []string
	RevokedTokens       map[string]bool
}

// NewMockSDK creates a MockSDK. AllowAll defaults to true (DenyAll=false).
func NewMockSDK(cfg MockConfig) *MockSDK {
	if cfg.Claims == nil {
		cfg.Claims = &Claims{
			Subject:  "test-user",
			TenantID: "test-tenant",
			Roles:    []string{"member"},
		}
	}
	return &MockSDK{
		Config:        &Config{FailMode: "open"},
		cfg:           cfg,
		AllowAll:      !cfg.DenyAll,
		Flags:         map[string]bool{},
		RevokedTokens: map[string]bool{},
	}
}

// Authorize validates a token using the (ctx, token) signature matching *SDK.
// Records the token for later assertion via AuthorizeCalls.
func (m *MockSDK) Authorize(_ context.Context, token string) (*Claims, error) {
	m.AuthorizeCalls = append(m.AuthorizeCalls, token)
	if m.cfg.DenyAll {
		return nil, fmt.Errorf("coresdk: mock denied")
	}
	return m.cfg.Claims, nil
}

// AuthorizeAction validates a token against a resource+action pair.
// Satisfies the three-arg form: Authorize(token, resource, action string) (*AuthDecision, error).
func (m *MockSDK) AuthorizeAction(token, resource, action string) (*AuthDecision, error) {
	m.AuthorizeCalls = append(m.AuthorizeCalls, token+"|"+resource+"|"+action)
	if m.cfg.DenyAll || !m.AllowAll {
		return &AuthDecision{Allowed: false}, fmt.Errorf("coresdk: mock denied")
	}
	return &AuthDecision{Allowed: true, Claims: m.cfg.Claims}, nil
}

// EvaluatePolicy evaluates a policy rule. Respects DenyAll.
func (m *MockSDK) EvaluatePolicy(_ context.Context, rule string, _ map[string]any) (bool, error) {
	m.PolicyCalls = append(m.PolicyCalls, rule)
	return !m.cfg.DenyAll && m.AllowAll, nil
}

// IsEnabled checks a feature flag by key and tenantID.
// Returns the value from the Flags map if set; otherwise returns AllowAll.
func (m *MockSDK) IsEnabled(flagKey, _ string) bool {
	if v, ok := m.Flags[flagKey]; ok {
		return v
	}
	return m.AllowAll
}

// RevokeToken mock — records the call and adds token to RevokedTokens.
func (m *MockSDK) RevokeToken(_ context.Context, token string) error {
	m.RevokeTokenCalls = append(m.RevokeTokenCalls, token)
	if m.cfg.DenyAll {
		return fmt.Errorf("coresdk: mock denied")
	}
	m.RevokedTokens[token] = true
	return nil
}

// IsRevoked mock — checks the RevokedTokens map.
func (m *MockSDK) IsRevoked(_ context.Context, token string) (bool, error) {
	m.IsRevokedCalls = append(m.IsRevokedCalls, token)
	if m.cfg.DenyAll {
		return false, fmt.Errorf("coresdk: mock denied")
	}
	return m.RevokedTokens[token], nil
}

// CheckRateLimit mock — returns allowed based on AllowAll.
func (m *MockSDK) CheckRateLimit(_ context.Context, key string) (*RateLimitDecision, error) {
	m.RateLimitCalls = append(m.RateLimitCalls, key)
	if m.cfg.DenyAll {
		return &RateLimitDecision{Allowed: false}, nil
	}
	return &RateLimitDecision{Allowed: true, Remaining: 100, ResetAt: 0}, nil
}

// EmitAuditEvent mock — records the call.
func (m *MockSDK) EmitAuditEvent(_ context.Context, action, userID, outcome string, _ map[string]string) error {
	m.AuditCalls = append(m.AuditCalls, action+"|"+userID+"|"+outcome)
	if m.cfg.DenyAll {
		return fmt.Errorf("coresdk: mock denied")
	}
	return nil
}

// EvaluateFlag mock — checks the Flags map, falls back to AllowAll.
func (m *MockSDK) EvaluateFlag(_ context.Context, key, _ string) (*FlagDecision, error) {
	m.FlagCalls = append(m.FlagCalls, key)
	if m.cfg.DenyAll {
		return &FlagDecision{Enabled: false, Key: key}, nil
	}
	enabled := m.AllowAll
	if v, ok := m.Flags[key]; ok {
		enabled = v
	}
	return &FlagDecision{Enabled: enabled, Key: key}, nil
}

// CheckEntitlement mock — returns allowed based on AllowAll.
func (m *MockSDK) CheckEntitlement(_ context.Context, key string) (*LicenseInfo, error) {
	m.EntitlementCalls = append(m.EntitlementCalls, key)
	if m.cfg.DenyAll {
		return &LicenseInfo{Allowed: false}, nil
	}
	return &LicenseInfo{Allowed: true, Plan: "enterprise", Features: []string{key}}, nil
}

// AssertNoPII fails t if any span attribute value contains unredacted PII-like patterns
// (email addresses detected by the presence of "@" in a value).
func AssertNoPII(t testing.TB, spans []MockSpan) {
	t.Helper()
	for _, span := range spans {
		for k, v := range span.Attributes {
			// email heuristic: contains "@" and a "." after it
			lower := strings.ToLower(v)
			if atIdx := strings.Index(lower, "@"); atIdx >= 0 {
				after := lower[atIdx+1:]
				if strings.Contains(after, ".") {
					t.Errorf("PII detected in span %q attribute %q: value looks like an email: %q",
						span.Name, k, v)
				}
			}
		}
	}
}
