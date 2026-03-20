package masking_test

import (
	"testing"

	"github.com/coresdk-dev/sdk-go/masking"
	"go.opentelemetry.io/otel/attribute"
)

func TestMaskValue_Email(t *testing.T) {
	result := masking.MaskValue("contact user@example.com now")
	if result != "[REDACTED]" {
		t.Errorf("expected [REDACTED], got %q", result)
	}
}

func TestMaskValue_BearerToken(t *testing.T) {
	result := masking.MaskValue("Bearer eyJhbGciOiJSUzI1NiJ9.abc.def")
	if result != "[REDACTED]" {
		t.Errorf("expected [REDACTED], got %q", result)
	}
}

func TestMaskValue_Clean(t *testing.T) {
	result := masking.MaskValue("GET /api/users 200")
	if result != "GET /api/users 200" {
		t.Errorf("clean value should be unchanged, got %q", result)
	}
}

func TestIsBlockedField(t *testing.T) {
	blocked := []string{"password", "secret", "authorization", "token"}
	for _, f := range blocked {
		if !masking.IsBlockedField(f) {
			t.Errorf("expected %q to be blocked", f)
		}
	}
	if masking.IsBlockedField("username") {
		t.Error("username should not be blocked")
	}
}

func TestMaskAttributes(t *testing.T) {
	attrs := []attribute.KeyValue{
		attribute.String("password", "hunter2"),
		attribute.String("http.method", "GET"),
		attribute.String("user.email", "alice@example.com"),
	}
	result := masking.MaskAttributes(attrs)
	for _, kv := range result {
		if kv.Value.AsString() == "hunter2" || kv.Value.AsString() == "alice@example.com" {
			t.Errorf("PII not redacted in attribute %s=%s", kv.Key, kv.Value.AsString())
		}
	}
}
