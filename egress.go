package coresdk

import (
	"context"
	"fmt"
	"net/http"
)

// HTTPClient is an http.Client wrapper that checks outbound URLs
// against the CoreSDK SSRF firewall before making requests.
type HTTPClient struct {
	SDK   *SDK
	Inner *http.Client
}

// NewHTTPClient creates an HTTP client with SSRF protection.
// If inner is nil, http.DefaultClient is used.
func NewHTTPClient(sdk *SDK, inner *http.Client) *HTTPClient {
	if inner == nil {
		inner = http.DefaultClient
	}
	return &HTTPClient{SDK: sdk, Inner: inner}
}

// Do checks the request URL against the SSRF firewall, then forwards to the inner client.
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	decision, err := c.SDK.CheckEgress(req.Context(), req.URL.String())
	if err != nil {
		// Fail-open: if egress check errors, allow the request
		return c.Inner.Do(req)
	}
	if !decision.Allowed {
		return nil, fmt.Errorf("CoreSDK SSRF firewall blocked %s: %s", req.URL, decision.Reason)
	}
	return c.Inner.Do(req)
}

// Get is a convenience method that sends a GET request with SSRF protection.
func (c *HTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}
