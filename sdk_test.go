package coresdk_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coresdk "github.com/coresdk/go-sdk"
	"github.com/coresdk/go-sdk/masking"
)

func TestMockSDK_AllowsByDefault(t *testing.T) {
	sdk := coresdk.NewMockSDK(coresdk.MockConfig{})
	claims, err := sdk.Authorize(context.Background(), "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims == nil {
		t.Fatal("expected claims, got nil")
	}
	if claims.Subject == "" {
		t.Error("expected non-empty subject")
	}
}

func TestMockSDK_DenyMode(t *testing.T) {
	sdk := coresdk.NewMockSDK(coresdk.MockConfig{DenyAll: true})
	_, err := sdk.Authorize(context.Background(), "token")
	if err == nil {
		t.Fatal("expected error when deny mode, got nil")
	}
}

func TestMockSDK_FailModeClosedNoClient(t *testing.T) {
	// SDK with FailMode=closed and no client should error
	sdk := &coresdk.SDK{Config: &coresdk.Config{FailMode: "closed"}}
	_, err := sdk.Authorize(context.Background(), "token")
	if err == nil {
		t.Fatal("expected error for fail-closed with no client")
	}
}

func TestMockSDK_FailModeOpenNoClient(t *testing.T) {
	// SDK with FailMode=open and no client should succeed with FailOpen=true
	sdk := &coresdk.SDK{Config: &coresdk.Config{FailMode: "open"}}
	claims, err := sdk.Authorize(context.Background(), "token")
	if err != nil {
		t.Fatalf("expected no error for fail-open, got: %v", err)
	}
	if !claims.FailOpen {
		t.Error("expected FailOpen=true")
	}
}

func TestMockSDK_TracksCalls(t *testing.T) {
	sdk := coresdk.NewMockSDK(coresdk.MockConfig{})
	sdk.Authorize(context.Background(), "tok-abc")
	sdk.Authorize(context.Background(), "tok-xyz")
	if len(sdk.AuthorizeCalls) != 2 {
		t.Errorf("expected 2 calls, got %d", len(sdk.AuthorizeCalls))
	}
}

func TestEvaluatePolicy_FailModeClosedNoClient(t *testing.T) {
	sdk := &coresdk.SDK{Config: &coresdk.Config{FailMode: "closed"}}
	_, err := sdk.EvaluatePolicy(context.Background(), "data.allow", map[string]any{"x": 1})
	if err == nil {
		t.Fatal("expected error for fail-closed policy eval with no client")
	}
}

func TestMockSDK_AuthorizeAllowAll(t *testing.T) {
	sdk := coresdk.NewMockSDK(coresdk.MockConfig{})
	decision, err := sdk.AuthorizeAction("tok", "posts", "read")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Error("expected Allowed=true when AllowAll=true")
	}
}

func TestMaskString_RedactsEmail(t *testing.T) {
	input := "contact user@example.com please"
	result := masking.MaskString(input)
	if result != "[REDACTED]" {
		t.Errorf("expected [REDACTED], got %q", result)
	}
	if strings.Contains(result, "@") {
		t.Error("email address not redacted from output")
	}
}

func TestProblemDetail_WriteHTTP(t *testing.T) {
	pd := coresdk.Unauthorized("bad token")
	w := httptest.NewRecorder()
	pd.WriteHTTP(w)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %q", ct)
	}
}
