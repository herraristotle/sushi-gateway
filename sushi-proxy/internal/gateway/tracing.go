package gateway

import (
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Global observability flags
var (
	EnableOTEL   = false
	OtelEndpoint = ""
	ServiceName  = "sushi-gateway"
)

// EnableTracing sets the global tracing flag and initializes the tracer
func EnableTracing(config map[string]interface{}) {
	slog.Info("OpenTelemetry tracing enabled via plugin")
	EnableOTEL = true

	if endpoint, ok := config["endpoint"].(string); ok {
		OtelEndpoint = endpoint
	}
	if svcName, ok := config["resource_attributes"].(map[string]interface{})["service.name"].(string); ok {
		ServiceName = svcName
	}
	// Fallback/Legacy config key
	if svcName, ok := config["service_name"].(string); ok {
		ServiceName = svcName
	}

	// Initialize basic stdout exporter for now as OTLP dep is missing,
	// but this structure allows easy swap to OTLP.
	initTracer()
}

func initTracer() {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		slog.Error("failed to initialize stdout trace exporter", "error", err)
		return
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(ServiceName),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	slog.Info("OTEL Gloabl Tracer initialized", "service", ServiceName)
}
