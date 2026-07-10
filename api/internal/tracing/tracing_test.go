package tracing

import (
	"context"
	"errors"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// noopExporter is a minimal SpanExporter that does nothing.
type noopExporter struct{}

func (n *noopExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}
func (n *noopExporter) Shutdown(_ context.Context) error { return nil }

// failShutdownExporter returns an error on Shutdown.
type failShutdownExporter struct{ noopExporter }

func (f *failShutdownExporter) Shutdown(_ context.Context) error {
	return errors.New("shutdown failed")
}

// saveAndRestore saves the current factory functions and returns a cleanup
// function that restores them.
func saveAndRestore() func() {
	origExporter := newExporter
	origResource := newResource
	return func() {
		newExporter = origExporter
		newResource = origResource
	}
}

func TestInit_ReturnsShutdownFunc(t *testing.T) {
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:19999")
	defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	ctx := context.Background()
	shutdown := Init(ctx)

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	shutdown(ctx)
}

func TestInit_ShutdownIsCallable(t *testing.T) {
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:19999")
	defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	ctx := context.Background()
	shutdown := Init(ctx)
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

func TestInit_ExporterCreationFailure(t *testing.T) {
	restore := saveAndRestore()
	defer restore()

	newExporter = func(_ context.Context) (sdktrace.SpanExporter, error) {
		return nil, errors.New("exporter creation failed")
	}

	ctx := context.Background()
	shutdown := Init(ctx)

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function even on exporter failure")
	}

	// Calling shutdown on the noop should not panic.
	shutdown(ctx)
}

func TestInit_ResourceCreationFailure(t *testing.T) {
	restore := saveAndRestore()
	defer restore()

	newExporter = func(_ context.Context) (sdktrace.SpanExporter, error) {
		return &noopExporter{}, nil
	}
	newResource = func(_ context.Context) (*resource.Resource, error) {
		return nil, errors.New("resource creation failed")
	}

	ctx := context.Background()
	shutdown := Init(ctx)

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function even on resource failure")
	}

	shutdown(ctx)
}

func TestInit_SuccessWithMock(t *testing.T) {
	restore := saveAndRestore()
	defer restore()

	newExporter = func(_ context.Context) (sdktrace.SpanExporter, error) {
		return &noopExporter{}, nil
	}
	newResource = func(_ context.Context) (*resource.Resource, error) {
		return resource.Default(), nil
	}

	ctx := context.Background()
	shutdown := Init(ctx)

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	tp := otel.GetTracerProvider()
	if tp == nil {
		t.Fatal("expected tracer provider to be set")
	}

	shutdown(ctx)
}

func TestInit_ShutdownError(t *testing.T) {
	restore := saveAndRestore()
	defer restore()

	newExporter = func(_ context.Context) (sdktrace.SpanExporter, error) {
		return &failShutdownExporter{}, nil
	}
	newResource = func(_ context.Context) (*resource.Resource, error) {
		return resource.Default(), nil
	}

	ctx := context.Background()
	shutdown := Init(ctx)

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// This exercises the error log path inside the shutdown closure.
	// The canceled context forces the tracer provider's Shutdown to fail.
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	shutdown(canceledCtx)
}
