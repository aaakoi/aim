package main

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// InitTracer 初始化链路追踪
// 如果设置了 OTEL_EXPORTER_OTLP_ENDPOINT，使用 OTLP 导出器
// 否则使用 stdout 导出器（用于开发调试）
func InitTracer(serviceName string) (func(context.Context) error, error) {
	var exporter sdktrace.SpanExporter
	var err error

	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	if otlpEndpoint != "" {
		// 生产环境：使用 OTLP 导出器（发送到 Jaeger/Zipkin 等）
		client := otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(otlpEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		exporter, err = otlptrace.New(context.Background(), client)
		if err != nil {
			return nil, err
		}
	} else {
		// 开发环境：输出到控制台
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
	}

	// 创建资源
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	// 创建 TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// 设置为全局 TracerProvider
	otel.SetTracerProvider(tp)

	// 设置传播器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// InitTracerSimple 简化版初始化（仅控制台输出）
func InitTracerSimple(serviceName string) (func(context.Context) error, error) {
	// 输出到控制台
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	// 创建资源
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	)

	// 创建 TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	// 返回安全的关闭函数
	return func(ctx context.Context) error {
		if tp != nil {
			return tp.Shutdown(ctx)
		}
		return nil
	}, nil
}
