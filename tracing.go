package coresdk

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const tracerName = "coresdk"

// Trace wraps a function in an OTel span with coresdk.intent attribute.
func Trace(ctx context.Context, intent string, fn func(ctx context.Context) error) error {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, intent)
	defer span.End()
	span.SetAttributes(attribute.String("coresdk.intent", intent))
	return fn(ctx)
}

// SetupOTel configures a basic OTel TracerProvider if none exists.
func SetupOTel(serviceName string) {
	if otel.GetTracerProvider() != nil {
		return
	}
	// TODO: configure OTLP exporter to sidecar
}
