package telemetry

import (
	"context"
	"io"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitTracer initializes the OpenTelemetry tracer provider.
func InitTracer(serviceName string) (func(context.Context) error, error) {
	// Use stdout exporter for demonstration/logging purposes.
	// In production, this would likely be an OTLP exporter (gRPC/HTTP).
	exporter, err := stdouttrace.New(
		stdouttrace.WithWriter(io.Discard), // Discard output by default to avoid cluttering logs unless debugging
		// To see traces in stdout, use: stdouttrace.WithWriter(os.Stdout),
		// or better: use stdouttrace.WithPrettyPrint()
	)
	if err != nil {
		return nil, err
	}

	// If OTEL_DEBUG is set, print traces to stdout
	if os.Getenv("OTEL_DEBUG") == "true" {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
	}

	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(r),
	)

	// Register the global tracer provider
	otel.SetTracerProvider(tp)

	// Set the global propagator to W3C TraceContext (standard distributed tracing headers)
	// This ensures we read/write 'traceparent' and 'tracestate' headers.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp.Shutdown, nil
}
