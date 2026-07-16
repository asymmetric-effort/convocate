package tracing

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// newExporter creates an OTLP trace exporter. It is a package-level variable
// so tests can replace it with a stub.
var newExporter = func(ctx context.Context) (sdktrace.SpanExporter, error) {
	return otlptracehttp.New(ctx)
}

// newResource creates the OTEL resource. It is a package-level variable so
// tests can replace it with a stub.
var newResource = func(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("convocate-api"),
		),
	)
}

// Init initializes the OpenTelemetry tracer provider with an OTLP HTTP
// exporter. It returns a shutdown function that should be called on exit.
func Init(ctx context.Context) func(context.Context) {
	exporter, err := newExporter(ctx)
	if err != nil {
		log.Printf("WARNING: failed to create OTLP trace exporter: %v", err)
		return func(context.Context) {}
	}

	res, err := newResource(ctx)
	if err != nil {
		log.Printf("WARNING: failed to create OTEL resource: %v", err)
		return func(context.Context) {}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return func(ctx context.Context) {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("WARNING: failed to shut down tracer provider: %v", err)
		}
	}
}
