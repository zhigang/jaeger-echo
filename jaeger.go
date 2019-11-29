package jaegerecho

import (
	"io"

	"github.com/labstack/echo/v4"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	log "github.com/sirupsen/logrus"
	jaeger "github.com/uber/jaeger-client-go"
	jaegerCfg "github.com/uber/jaeger-client-go/config"
)

// JaegerTracer middleware adds a `Span` to the request.
func JaegerTracer() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tracer := opentracing.GlobalTracer()
			// if tracer not found, skip.
			if tracer == nil {
				return next(c)
			}

			req := c.Request()
			ctx := req.Context()

			// 创建 rootSpan
			var rootCtx opentracing.SpanContext
			if rootSpan := opentracing.SpanFromContext(ctx); rootSpan != nil {
				rootCtx = rootSpan.Context()
			}

			span := tracer.StartSpan(
				req.URL.Path,
				opentracing.ChildOf(rootCtx),
				ext.SpanKindRPCClient,
			)
			defer span.Finish()
			ext.HTTPMethod.Set(span, req.Method)
			ext.HTTPUrl.Set(span, req.URL.RequestURI())
			// make the Span current in the context
			ctx = opentracing.ContextWithSpan(ctx, span)

			c.SetRequest(req.WithContext(ctx))

			if err := next(c); err != nil {
				log.Error(err)
			}

			res := c.Response()

			ext.HTTPStatusCode.Set(span, uint16(res.Status))

			return nil
		}
	}
}

// NewJaegerTracer is returns a jaejer tracer.
// More config: https://github.com/jaegertracing/jaeger-client-go/blob/master/config/config.go
func NewJaegerTracer(serviceName, endpoint string, options ...jaegerCfg.Option) (opentracing.Tracer, io.Closer, error) {
	cfg := jaegerCfg.Configuration{
		ServiceName: serviceName,
		Sampler: &jaegerCfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1.0,
		},
		Reporter: &jaegerCfg.ReporterConfig{
			LogSpans: false,
			// BufferFlushInterval: 1 * time.Second,
			CollectorEndpoint: "http://localhost:14268/api/traces",
		},
	}

	if endpoint != "" {
		cfg.Reporter.CollectorEndpoint = endpoint
	}

	// Example logger and metrics factory. Use github.com/uber/jaeger-client-go/log
	// and github.com/uber/jaeger-lib/metrics respectively to bind to real logging and metrics
	// frameworks.
	// jLogger := jaeger.NullLogger
	// jMetricsFactory := metrics.NullFactory
	// jaegerCfg.Logger(jLogger),
	// jaegerCfg.Metrics(jMetricsFactory),

	tracer, closer, err := cfg.NewTracer(
		options...,
	)

	if err != nil {
		log.Errorf("create tracer error: %s", err.Error())
	}

	opentracing.SetGlobalTracer(tracer)

	return tracer, closer, err
}

// StartSpanFromEchoContext returns the `Span` previously associated with `echo.Context`, or
// new span if `echo.Context` no span could be found, or `nil` if no tracer could be found.
func StartSpanFromEchoContext(ctx echo.Context, operationName string) opentracing.Span {
	tracer := opentracing.GlobalTracer()
	if tracer == nil {
		return nil
	}
	var parentCtx opentracing.SpanContext
	if parentSpan := opentracing.SpanFromContext(ctx.Request().Context()); parentSpan != nil {
		parentCtx = parentSpan.Context()
	}

	span := tracer.StartSpan(
		operationName,
		opentracing.ChildOf(parentCtx),
		ext.SpanKindRPCClient,
	)
	if parentCtx == nil {
		span.SetTag("parent.context", "nil")
	}
	return span
}

// StartSpanFromContext returns the `Span` previously associated with `ctx`, or
// new span if `ctx` no span could be found, or `nil` if no tracer could be found.
func StartSpanFromContext(ctx opentracing.SpanContext, operationName string) opentracing.Span {
	tracer := opentracing.GlobalTracer()
	if tracer == nil {
		return nil
	}

	span := tracer.StartSpan(
		operationName,
		opentracing.ChildOf(ctx),
		ext.SpanKindRPCClient,
	)

	if ctx == nil {
		span.SetTag("parent.context", "nil")
	}

	return span
}
