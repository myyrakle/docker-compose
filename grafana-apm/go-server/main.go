package main

import (
	"context"
	"fmt"
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
	otelTrace "go.opentelemetry.io/otel/trace"
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
			semconv.ServiceNamespaceKey.String("dev"),
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

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("go-server"),
			semconv.ServiceNamespaceKey.String("dev"),
		)),
	)
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

	tracer := traceProvider.Tracer("foo")

	defer func() {
		if err := traceProvider.Shutdown(context.Background()); err != nil {
			log.Fatalf("failed to shutdown tracer: %v", err)
		}
	}()

	e := echo.New()

	e.Use(otelecho.Middleware("go-server"))

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			span := otelTrace.SpanFromContext(c.Request().Context())

			// responseStatusCode := c.Response().Status
			if span != nil {
				// spanStatus := codes.OK

				// switch responseStatusCode {
				// case http.StatusOK:
				// 	spanStatus = codes.OK
				// case http.StatusInternalServerError:
				// 	spanStatus = codes.Internal
				// case http.StatusNotFound:
				// 	spanStatus = codes.NotFound
				// }

				fmt.Println("!!!@@")
				// span.SetStatus(codes.Error, "error")

				// fmt.Println(c.Response().Status)
				// span.SetAttributes(attribute.Int("status", codes.Error))
			}
			return err
		}
	})

	e.GET("/trace", func(c echo.Context) error {
		_, span := tracer.Start(c.Request().Context(), "test-span")
		time.Sleep(200 * time.Millisecond)
		span.End()

		_, span = tracer.Start(c.Request().Context(), "test-span2")
		time.Sleep(500 * time.Millisecond)
		span.End()

		return c.String(http.StatusOK, "Hello, World!")
	})

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

	e.GET("/not-found", func(c echo.Context) error {
		time.Sleep(2 * time.Second)
		return c.String(http.StatusNotFound, "asdf")
	})

	e.GET("/internal", func(c echo.Context) error {
		time.Sleep(2 * time.Second)
		return c.String(http.StatusInternalServerError, "asdf")
	})

	e.Logger.Fatal(e.Start(":1323"))
}
