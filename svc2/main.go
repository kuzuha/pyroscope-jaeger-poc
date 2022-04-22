package main

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/pyroscope-io/client/pyroscope"
	"github.com/pyroscope-io/otelpyroscope"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"log"
	"net/http"
)

var tracer trace.Tracer

func main() {
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
		jaeger.WithEndpoint("http://localhost:14268/api/traces"),
	))
	if err != nil {
		return
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(otelpyroscope.NewTracerProvider(tp,
		otelpyroscope.WithAppName("svc2"),
		otelpyroscope.WithPyroscopeURL("http://pyroscope:4040"),
		otelpyroscope.WithRootSpanOnly(true),
		otelpyroscope.WithAddSpanName(true),
		otelpyroscope.WithProfileURL(true),
		otelpyroscope.WithProfileBaselineURL(true),
	))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatal(err)
		}
	}()
	tracer = tp.Tracer("svc2")

	p, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "svc2",
		ServerAddress:   "http://pyroscope:4040",
		Logger:          pyroscope.StandardLogger,
	})
	if err != nil {
		pyroscope.StandardLogger.Errorf(err.Error())
	}
	defer func(p *pyroscope.Profiler) {
		_ = p.Stop()
	}(p)

	e := echo.New()
	e.Use(otelecho.Middleware("svc2"))
	e.GET("/greeting", func(c echo.Context) error {
		_, span := tracer.Start(c.Request().Context(), "handler")
		defer span.End()
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.Logger.Fatal(e.Start(":1324"))
}
