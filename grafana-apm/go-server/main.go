package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

	return traceProvider
}

func initMetric() *metric.MeterProvider {
	ctx := context.Background()

	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(
		"http://localhost:4318",
	), otlpmetrichttp.WithInsecure())
	if err != nil {
		panic(err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(3*time.Second))),
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

	return meterProvider
}

func initDB() *sql.DB {
	connStr := "postgres://otel:otel@localhost:25432/otel?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}

	// Test connection
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	return db
}

func main() {
	traceProvider := initTracer()
	_ = initMetric()

	tracer := traceProvider.Tracer("foo")

	defer func() {
		if err := traceProvider.Shutdown(context.Background()); err != nil {
			log.Fatalf("failed to shutdown tracer: %v", err)
		}
	}()

	// meter := metricProvider.Meter("db-operations")
	// dbDurationHistogram, err := meter.Float64Histogram(
	// 	"db_client_operation_duration_seconds",
	// )
	// if err != nil {
	// 	log.Fatalf("Failed to create metric: %v", err)
	// }

	db := initDB()

	executeQuery := func(ctx context.Context, methodName string, query string) (*sql.Rows, error) {
		_, span := tracer.Start(ctx, "db-span")
		defer span.End()

		// https://opentelemetry.io/docs/specs/semconv/database/database-metrics/
		span.SetAttributes(attribute.String("db.database", "none"))
		span.SetAttributes(attribute.String("db.system.name", "postgresql"))
		span.SetAttributes(attribute.String("db.operation", "query"))
		span.SetAttributes(attribute.String("db.operation.name", methodName))
		span.SetAttributes(attribute.String("db.query.text", query))

		result, err := db.Query(query)

		if err != nil {
			span.SetAttributes(attribute.String("db.error", err.Error()))
			span.RecordError(err)
			return nil, err
		}
		return result, nil
	}

	e := echo.New()

	e.Use(otelecho.Middleware("go-server"))

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			span := otelTrace.SpanFromContext(c.Request().Context())

			if span != nil {
				if err != nil {
					span.SetStatus(codes.Error, err.Error())
					return err
				}

				responseStatusCode := c.Response().Status

				spanStatus := codes.Ok

				if responseStatusCode >= 400 {
					spanStatus = codes.Error
				}

				span.SetStatus(spanStatus, "")
				span.SetAttributes(attribute.Bool("primary", true))
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

		return c.String(http.StatusOK, "trace completed")
	})

	e.GET("/api", func(c echo.Context) error {
		// call google.com
		response, _ := http.Get("https://google.com")
		defer response.Body.Close()

		return c.String(http.StatusOK, "api called")
	})

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.GET("/foo", func(c echo.Context) error {
		time.Sleep(1 * time.Second)

		span := otelTrace.SpanFromContext(c.Request().Context())

		if span != nil {
			span.SetAttributes(attribute.KeyValue{
				Key:   "password",
				Value: attribute.StringValue("q1w2e3r4"),
			})
		}

		return c.String(http.StatusOK, "foo")
	})

	e.GET("/bar", func(c echo.Context) error {
		time.Sleep(2 * time.Second)
		return c.String(http.StatusOK, "bar")
	})

	e.GET("/not-found", func(c echo.Context) error {
		time.Sleep(2 * time.Second)
		return c.String(http.StatusNotFound, "NOT FOUND")
	})

	e.GET("/internal", func(c echo.Context) error {
		time.Sleep(2 * time.Second)
		return c.String(http.StatusInternalServerError, "INTERNAL SERVER ERROR")
	})

	e.GET("/too-slow", func(c echo.Context) error {
		time.Sleep(5 * time.Second)
		return c.String(http.StatusInternalServerError, "slow")
	})

	e.GET("/db-call", func(c echo.Context) error {
		_, err := executeQuery(c.Request().Context(), "just select 1", "SELECT 1")
		if err != nil {
			c.String(http.StatusInternalServerError, "db error")
		}

		return c.String(http.StatusOK, "db called")
	})

	e.GET("/db-error", func(c echo.Context) error {
		_, err := executeQuery(c.Request().Context(), "just error", "SELECT asdf")
		if err != nil {
			c.String(http.StatusInternalServerError, "db error")
			return nil
		}

		return c.String(http.StatusOK, "db error")
	})

	e.Logger.Fatal(e.Start(":1323"))
}
