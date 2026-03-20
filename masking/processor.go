// Package masking provides PII redaction for OTel spans and log entries.
package masking

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var blockedFields = map[string]bool{
	"password": true, "passwd": true, "secret": true, "token": true,
	"api_key": true, "apikey": true, "authorization": true, "auth": true,
	"private_key": true, "credential": true, "credentials": true,
	"access_key": true, "access_token": true, "refresh_token": true,
	"client_secret": true, "ssn": true,
}

var (
	jwtRe    = regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`)
	bearerRe = regexp.MustCompile(`(?i)Bearer\s+\S+`)
	emailRe  = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
)

const redacted = "[REDACTED]"

// MaskValue redacts PII from a string value.
func MaskValue(v string) string {
	if jwtRe.MatchString(v) || bearerRe.MatchString(v) || emailRe.MatchString(v) {
		return redacted
	}
	return v
}

// IsBlockedField returns true if the field name should always be redacted.
func IsBlockedField(name string) bool {
	return blockedFields[strings.ToLower(name)]
}

// PIIMaskingSpanProcessor redacts PII from span attributes before export.
// Register as SpanProcessor (not SpanExporter) so masking fires before queue.
type PIIMaskingSpanProcessor struct{}

// NewPIIMaskingSpanProcessor creates a SpanProcessor that redacts PII from span attributes.
func NewPIIMaskingSpanProcessor() *PIIMaskingSpanProcessor {
	return &PIIMaskingSpanProcessor{}
}

// OnStart fires when a span is created. ReadWriteSpan is mutable — we redact
// any PII-bearing attributes that were set at span creation time.
func (p *PIIMaskingSpanProcessor) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	masked := MaskAttributes(s.Attributes())
	// Replace all attributes with the masked set.
	// SetAttributes overwrites existing keys.
	s.SetAttributes(masked...)
}

// OnEnd fires after the span ends. ReadOnlySpan is immutable; masking is a
// no-op here — all redaction happens in OnStart.
func (p *PIIMaskingSpanProcessor) OnEnd(_ sdktrace.ReadOnlySpan) {}

func (p *PIIMaskingSpanProcessor) Shutdown(_ context.Context) error   { return nil }
func (p *PIIMaskingSpanProcessor) ForceFlush(_ context.Context) error { return nil }

// MaskAttributes redacts PII from a map of span attributes.
func MaskAttributes(attrs []attribute.KeyValue) []attribute.KeyValue {
	result := make([]attribute.KeyValue, 0, len(attrs))
	for _, kv := range attrs {
		key := string(kv.Key)
		switch {
		case IsBlockedField(key):
			result = append(result, attribute.String(key, redacted))
		case kv.Value.Type() == attribute.STRING:
			masked := MaskValue(kv.Value.AsString())
			result = append(result, attribute.String(key, masked))
		default:
			result = append(result, kv)
		}
	}
	return result
}
