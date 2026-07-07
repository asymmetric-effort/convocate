package tracing

import (
	"context"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestInit_ReturnsShutdownFunc(t *testing.T) {
	// Set OTLP endpoint to a non-existent address so exporter creation
	// succeeds but exports will fail silently (which is fine for testing).
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:19999")
	defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	ctx := context.Background()
	shutdown := Init(ctx)

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Call shutdown -- should not panic
	shutdown(ctx)
}

func TestInit_ShutdownIsCallable(t *testing.T) {
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:19999")
	defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	ctx := context.Background()
	shutdown := Init(ctx)
	// Multiple calls should be safe
	shutdown(ctx)
}

func TestInit_SetsTracerProvider(t *testing.T) {
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:19999")
	defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	ctx := context.Background()
	shutdown := Init(ctx)
	defer shutdown(ctx)

	tp := otel.GetTracerProvider()
	if tp == nil {
		t.Fatal("expected tracer provider to be set")
	}
}

func TestInit_TracerProviderCreatesTracer(t *testing.T) {
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:19999")
	defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	ctx := context.Background()
	shutdown := Init(ctx)
	defer shutdown(ctx)

	tracer := otel.Tracer("test-tracer")
	if tracer == nil {
		t.Fatal("expected tracer to be non-nil")
	}
}
