package tracer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"web3/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/credentials"
)

// Manager 持有 Web3 进程的 TracerProvider，并负责退出时刷新尚未导出的 Span。
type Manager struct {
	provider trace.TracerProvider
	tracer   trace.Tracer
	shutdown func(context.Context) error
}

// NewTracerProvider 创建 OTLP/gRPC Trace Provider。
// 即使关闭导出，也会安装 W3C TraceContext/Baggage 传播器，保证上下游上下文格式一致。
func NewTracerProvider(ctx context.Context, cfg config.TracingConfig) (*Manager, error) {
	if ctx == nil {
		return nil, errors.New("tracing context is required")
	}
	installPropagator()
	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "web3-service"
	}

	if !cfg.Enabled {
		provider := noop.NewTracerProvider()
		otel.SetTracerProvider(provider)
		return &Manager{
			provider: provider,
			tracer:   provider.Tracer(serviceName),
			shutdown: func(context.Context) error { return nil },
		}, nil
	}

	sampler, err := createSampler(cfg)
	if err != nil {
		return nil, err
	}
	exporterOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(strings.TrimSpace(cfg.Endpoint)),
	}
	if cfg.UseTLS {
		tlsConfig, err := createTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		exporterOptions = append(exporterOptions, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		exporterOptions = append(exporterOptions, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, exporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("create Web3 OTLP trace exporter: %w", err)
	}
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, errors.Join(err, exporter.Shutdown(ctx))
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(provider)
	return &Manager{
		provider: provider,
		tracer:   provider.Tracer(serviceName),
		shutdown: provider.Shutdown,
	}, nil
}

func installPropagator() {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

func createSampler(cfg config.TracingConfig) (sdktrace.Sampler, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.SamplerType)) {
	case "const":
		if cfg.SamplerParam == 0 {
			return sdktrace.ParentBased(sdktrace.NeverSample()), nil
		}
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), nil
	case "probabilistic", "ratelimiting":
		if cfg.SamplerParam < 0 || cfg.SamplerParam > 1 {
			return nil, errors.New("tracing sampler parameter must be between 0 and 1")
		}
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerParam)), nil
	default:
		return nil, fmt.Errorf("unsupported tracing sampler type %q", cfg.SamplerType)
	}
}

func createTLSConfig(cfg config.TracingConfig) (*tls.Config, error) {
	caFile := strings.TrimSpace(cfg.CAFile)
	if caFile == "" {
		return nil, errors.New("tracing CA certificate is required when TLS is enabled")
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read tracing CA certificate: %w", err)
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("tracing CA certificate contains no valid certificate")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
		ServerName: strings.TrimSpace(cfg.ServerName),
	}
	hasCertificate := strings.TrimSpace(cfg.CertFile) != ""
	hasKey := strings.TrimSpace(cfg.KeyFile) != ""
	if hasCertificate != hasKey {
		return nil, errors.New("tracing client certificate and key must be configured together")
	}
	if hasCertificate {
		certificate, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load tracing client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, nil
}

func (m *Manager) Provider() trace.TracerProvider {
	if m == nil {
		return noop.NewTracerProvider()
	}
	return m.provider
}

func (m *Manager) Tracer() trace.Tracer {
	if m == nil || m.tracer == nil {
		return noop.NewTracerProvider().Tracer("web3-service")
	}
	return m.tracer
}

func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil || m.shutdown == nil {
		return nil
	}
	return m.shutdown(ctx)
}
