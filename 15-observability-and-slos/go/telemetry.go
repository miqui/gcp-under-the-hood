package main

import (
	"context"
	"os"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitTracing installs a global TracerProvider that exports spans to Cloud
// Trace using the Google Cloud exporter (Cloud Trace API v2). With
// OTEL_DISABLED=true (local runs/tests) it installs only propagation and a
// no-export provider so trace IDs still exist for log correlation.
func InitTracing(ctx context.Context, projectID string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if os.Getenv("OTEL_DISABLED") == "true" || projectID == "" {
		tp := sdktrace.NewTracerProvider()
		otel.SetTracerProvider(tp)
		return tp.Shutdown, nil
	}
	exp, err := texporter.New(texporter.WithProjectID(projectID))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
