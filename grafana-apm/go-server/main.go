package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func initTracer() *trace.TracerProvider {
	ctx := context.Background()

	// Create the OTLP traceExporter
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(
		"http://localhost:4318",
	), otlptracehttp.WithInsecure())
	if err != nil {
		panic(err)
	}

	// Create the tracer provider
	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("go-server"),
		)),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(traceProvider)

	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(
		"http://localhost:4318",
	), otlpmetrichttp.WithInsecure())
	if err != nil {
		panic(err)
	}

	meterProvider := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(metricExporter)))
	if err != nil {
		panic(err)
	}
	otel.SetMeterProvider(meterProvider)

	err = runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
	if err != nil {
		panic(err)
	}

	return traceProvider
}

func main() {
	traceProvider := initTracer()
	defer func() {
		if err := traceProvider.Shutdown(context.Background()); err != nil {
			log.Fatalf("failed to shutdown tracer: %v", err)
		}
	}()

	e := echo.New()

	e.Use(otelecho.Middleware("go-server"))

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.GET("/foo", func(c echo.Context) error {
		time.Sleep(1 * time.Second)
		return c.String(http.StatusOK, "foo")
	})

	e.GET("/bar", func(c echo.Context) error {
		time.Sleep(2 * time.Second)
		return c.String(http.StatusOK, "bar")
	})

	e.Logger.Fatal(e.Start(":1323"))
}
