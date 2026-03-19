package masking

import (
	"context"
	"log/slog"
)

// MaskingSlogHandler wraps slog.Handler to redact PII from log records.
type MaskingSlogHandler struct {
	inner slog.Handler
}

func NewMaskingSlogHandler(inner slog.Handler) *MaskingSlogHandler {
	return &MaskingSlogHandler{inner: inner}
}

func (h *MaskingSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *MaskingSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	masked := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if IsBlockedField(a.Key) {
			masked.AddAttrs(slog.String(a.Key, redacted))
		} else if a.Value.Kind() == slog.KindString {
			masked.AddAttrs(slog.String(a.Key, MaskValue(a.Value.String())))
		} else {
			masked.AddAttrs(a)
		}
		return true
	})
	return h.inner.Handle(ctx, masked)
}

func (h *MaskingSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MaskingSlogHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *MaskingSlogHandler) WithGroup(name string) slog.Handler {
	return &MaskingSlogHandler{inner: h.inner.WithGroup(name)}
}
