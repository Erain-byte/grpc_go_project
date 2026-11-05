package tracer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"
	"strings"
	"user/internal/config"
	"user/pkg/apperror"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/credentials"
)

type Manager struct {
	provider trace.TracerProvider
	tracer   trace.Tracer
	shutdown func(ctx context.Context) error
}

// 构造函数
func NewTracerProvider(ctx context.Context, cfg config.TracingConfig) (*Manager, error) {
	if ctx == nil {
		return nil, apperror.InvalidArgument("context is required")
	}
	installPropagator()
	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "user-service"
	}
	if !cfg.Enabled {
		provider := noop.NewTracerProvider()
		otel.SetTracerProvider(provider)
		return &Manager{
			provider: provider,
			tracer:   provider.Tracer(serviceName),
			shutdown: func(ctx context.Context) error { return nil },
		}, nil
	}
	//设置 option
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.UseTLS {
		tlsConfig, err := createTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig))) //使用TLS
	} else {
		//不使用TLS
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	//设置 exporter
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to create trace exporter",
			http.StatusInternalServerError,
		)
	}
	res, err := resource.New(
		ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)), //设置资源属性
	)
	if err != nil {
		return nil, errors.Join(err, exporter.Shutdown(ctx))
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(createSampler(cfg)),
	)
	//设置全局
	otel.SetTracerProvider(provider)
	return &Manager{
		provider: provider,
		tracer:   provider.Tracer(serviceName),
		// 关闭 Provider 才会先刷新 BatchSpanProcessor，再关闭 exporter。
		shutdown: provider.Shutdown,
	}, nil
}

func installPropagator() {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

func (m *Manager) Provider() trace.TracerProvider {
	if m == nil || m.provider == nil {
		return noop.NewTracerProvider()
	}
	return m.provider
}
func (m *Manager) Tracer() trace.Tracer {
	if m == nil || m.tracer == nil {
		return noop.NewTracerProvider().Tracer("user-service")
	}
	return m.tracer
}

// 关闭tracer
func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil || m.shutdown == nil {
		return nil
	}
	return m.shutdown(ctx)
}

// 采样器函数
func createSampler(cfg config.TracingConfig) sdktrace.Sampler {
	switch strings.ToLower(strings.TrimSpace(cfg.SamplerType)) {
	case "const":
		if cfg.SamplerParam == 0 {
			return sdktrace.ParentBased(sdktrace.NeverSample())
		}
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "probabilistic", "ratelimiting":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerParam))
	default:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}

// createTLSConfig创建证书凭证
func createTLSConfig(cfg config.TracingConfig) (*tls.Config, error) {
	//获取证书
	if cfg.CAFile == "" {
		return nil, apperror.InvalidArgument(
			"tracing CA certificate file is required when TLS is enabled",
		)
	}
	//读取证书
	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to read tracing CA certificate",
			http.StatusInternalServerError,
		)
	}
	//创建证书池
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, apperror.InvalidArgument("failed to append CA certificate")
	}
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    caCertPool,
		ServerName: cfg.ServerName,
	}
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, apperror.Wrap(
				err,
				apperror.CodeInternal,
				"failed to load tracing client certificate",
				http.StatusInternalServerError,
			)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil

}
