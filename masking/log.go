package masking

import (
	"context"
	"log/slog"
)

// SlogHandler wraps slog.Handler to redact PII from log records before they are written.
type SlogHandler struct {
	inner slog.Handler
}

// NewSlogHandler wraps inner with PII redaction applied to every log record.
func NewSlogHandler(inner slog.Handler) *SlogHandler {
	return &SlogHandler{inner: inner}
}

// Enabled reports whether the handler handles records at the given level.
func (h *SlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle redacts PII from record attributes before delegating to the inner handler.
func (h *SlogHandler) Handle(ctx context.Context, r slog.Record) error {
	masked := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		switch {
		case IsBlockedField(a.Key):
			masked.AddAttrs(slog.String(a.Key, redacted))
		case a.Value.Kind() == slog.KindString:
			masked.AddAttrs(slog.String(a.Key, MaskValue(a.Value.String())))
		default:
			masked.AddAttrs(a)
		}
		return true
	})
	return h.inner.Handle(ctx, masked)
}

// WithAttrs returns a new SlogHandler with the given attributes pre-set.
func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SlogHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup returns a new SlogHandler that qualifies keys with the given group name.
func (h *SlogHandler) WithGroup(name string) slog.Handler {
	return &SlogHandler{inner: h.inner.WithGroup(name)}
}
