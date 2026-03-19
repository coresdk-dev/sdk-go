// Package v1 contains hand-written proto stubs for coresdk.v1.
// Replace with buf-generated code when protoc/buf CLI is available in CI.
//
// Message shapes mirror proto/coresdk/v1/auth.proto and
// proto/coresdk/v1/common.proto exactly.
package v1

// ProblemDetail mirrors coresdk.v1.ProblemDetail (RFC 9457).
type ProblemDetail struct {
	Type       string            `json:"type,omitempty"`
	Title      string            `json:"title,omitempty"`
	Status     uint32            `json:"status,omitempty"`
	Detail     string            `json:"detail,omitempty"`
	Instance   string            `json:"instance,omitempty"`
	Extensions map[string]string `json:"extensions,omitempty"`
}

// TenantContext mirrors coresdk.v1.TenantContext.
type TenantContext struct {
	TenantID   string            `json:"tenant_id"`
	TenantName string            `json:"tenant_name,omitempty"`
	Roles      []string          `json:"roles,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// RequestMetadata mirrors coresdk.v1.RequestMetadata.
type RequestMetadata struct {
	RequestID   string `json:"request_id,omitempty"`
	TraceID     string `json:"trace_id,omitempty"`
	SpanID      string `json:"span_id,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

// ValidateTokenRequest mirrors coresdk.v1.ValidateTokenRequest.
type ValidateTokenRequest struct {
	Token            string           `json:"token"`
	Tenant           *TenantContext   `json:"tenant,omitempty"`
	Metadata         *RequestMetadata `json:"metadata,omitempty"`
	ExpectedAudience string           `json:"expected_audience,omitempty"`
}

// ValidateTokenResponse mirrors coresdk.v1.ValidateTokenResponse.
type ValidateTokenResponse struct {
	Valid      bool              `json:"valid"`
	Subject    string            `json:"subject"`
	Roles      []string          `json:"roles"`
	Claims     map[string]string `json:"claims"`
	ExpiresAt  int64             `json:"expires_at"`
	Error      *ProblemDetail    `json:"error,omitempty"`
}

// AuthorizeRequest mirrors coresdk.v1.AuthorizeRequest.
type AuthorizeRequest struct {
	Subject  string            `json:"subject"`
	Action   string            `json:"action"`
	Resource string            `json:"resource"`
	Tenant   *TenantContext    `json:"tenant,omitempty"`
	Metadata *RequestMetadata  `json:"metadata,omitempty"`
	Context  map[string]string `json:"context,omitempty"`
}

// AuthorizeResponse mirrors coresdk.v1.AuthorizeResponse.
type AuthorizeResponse struct {
	Allowed bool           `json:"allowed"`
	Reason  string         `json:"reason"`
	Error   *ProblemDetail `json:"error,omitempty"`
}

// GetJwksRequest mirrors coresdk.v1.GetJwksRequest.
type GetJwksRequest struct {
	Tenant *TenantContext `json:"tenant,omitempty"`
}

// GetJwksResponse mirrors coresdk.v1.GetJwksResponse.
type GetJwksResponse struct {
	JwksJSON string `json:"jwks_json"`
}
