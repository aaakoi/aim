package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var logger *zap.Logger

// InitLogger 初始化日志
func InitLogger() *zap.Logger {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
	return logger
}

// GetLogger 获取日志实例
func GetLogger() *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}
	return logger
}

// PrometheusMiddleware Prometheus指标中间件
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		endpoint := r.URL.Path
		method := r.Method

		// 包装ResponseWriter以获取状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(wrapped, r)

		// 记录指标
		duration := time.Since(start).Seconds()
		status := http.StatusText(wrapped.statusCode)

		RecordHttpRequest(endpoint, method, status)
		RecordHttpLatency(endpoint, method, duration)
	})
}

// TracingMiddleware 链路追踪中间件
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.GetTracerProvider().Tracer("aim-server")
		_, span := tracer.Start(r.Context(), r.URL.Path)
		defer span.End()

		// 添加属性
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
		)

		// 记录日志（带trace_id）
		traceID := span.SpanContext().TraceID().String()
		GetLogger().Info("HTTP request",
			zap.String("trace_id", traceID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(wrapped, r)

		GetLogger().Info("request completed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", wrapped.statusCode),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

// responseWriter 包装http.ResponseWriter以获取状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// MetricsHandler 返回Prometheus指标处理器
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// ========== 链路追踪辅助函数 ==========

// StartSpan 开始一个追踪span
func StartSpan(name string) (trace.Span, func()) {
	tracer := otel.GetTracerProvider().Tracer("aim-server")
	_, span := tracer.Start(nil, name)
	return span, func() { span.End() }
}

// SpanWithCtx 带上下文的span
func SpanWithCtx(ctx interface{ Done() <-chan struct{} }, name string) (trace.Span, func()) {
	tracer := otel.GetTracerProvider().Tracer("aim-server")
	// 如果ctx有context.Context接口，使用它
	_, span := tracer.Start(nil, name)
	return span, func() { span.End() }
}
