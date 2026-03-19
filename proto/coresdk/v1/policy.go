// Package v1 — hand-written proto stubs for coresdk.v1.PolicyService.
// Replace with buf-generated code when protoc/buf CLI is available in CI.
//
// Message shapes mirror proto/coresdk/v1/policy.proto exactly.
package v1

// PolicyEvaluateRequest mirrors coresdk.v1.PolicyEvaluateRequest.
type PolicyEvaluateRequest struct {
	Rule      string           `json:"rule"`       // e.g. "data.authz.allow"
	InputJSON string           `json:"input_json"` // JSON-encoded input document
	Tenant    *TenantContext   `json:"tenant,omitempty"`
	Metadata  *RequestMetadata `json:"metadata,omitempty"`
}

// PolicyEvaluateResponse mirrors coresdk.v1.PolicyEvaluateResponse.
type PolicyEvaluateResponse struct {
	Result  bool           `json:"result"`
	Reason  string         `json:"reason"`
	DryRun  bool           `json:"dry_run"`
	Error   *ProblemDetail `json:"error,omitempty"`
}

// WatchPolicyUpdatesRequest mirrors coresdk.v1.WatchPolicyUpdatesRequest.
type WatchPolicyUpdatesRequest struct {
	Tenant              *TenantContext `json:"tenant,omitempty"`
	LastBundleVersion   string         `json:"last_bundle_version,omitempty"`
}

// PolicyBundleUpdate mirrors coresdk.v1.PolicyBundleUpdate.
type PolicyBundleUpdate struct {
	BundleVersion string `json:"bundle_version"`
	BundleData    []byte `json:"bundle_data"`
	UpdatedAt     int64  `json:"updated_at"`
}
