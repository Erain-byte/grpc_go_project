package tracer

import (
	"admin/internal/config"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"gateway/pkg/apperror"
	"net/http"
	"os"

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
	shutdown func(context.Context) error
}

func NewTracerProvider(ctx context.Context, cfg config.TracingConfig) (*Manager, error) {
	if !cfg.Enabled {
		provider := noop.NewTracerProvider()
		return &Manager{
			provider: provider,
			tracer:   provider.Tracer(cfg.ServiceName),
			shutdown: func(ctx context.Context) error { return nil },
		}, nil
	}
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint), //设置OTLP exporter的地址
	}
	if cfg.UseTLS {
		tlsConfig, err := createTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig))) //使用TLS
	} else {
		opts = append(opts, otlptracegrpc.WithInsecure()) //不使用TLS
	}
	exporter, err := otlptracegrpc.New(ctx, opts...) //创建OTLP exporter
	if err != nil {
		return nil, apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to create OTLP trace exporter",
			http.StatusInternalServerError,
		)
	}
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, errors.Join(err, exporter.Shutdown(ctx))
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(createSampler(cfg)),
	)
	otel.SetTracerProvider(provider) //设置全局的tracer provider
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return &Manager{
		provider: provider,
		tracer:   provider.Tracer(cfg.ServiceName),
		shutdown: provider.Shutdown,
	}, nil
}

func createSampler(cfg config.TracingConfig) sdktrace.Sampler {
	switch cfg.SamplerType {
	case "const":
		if cfg.SamplerParam == 0 {
			return sdktrace.NeverSample()
		}
		return sdktrace.AlwaysSample()
	case "probabilistic":
		return sdktrace.TraceIDRatioBased(cfg.SamplerParam)
	case "ratelimiting":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerParam))
	default:
		return sdktrace.AlwaysSample()
	}
}
func createTLSConfig(cfg config.TracingConfig) (*tls.Config, error) {
	caFile := cfg.CAFile
	if caFile == "" {
		caFile = "/etc/certs/admin/ca.crt" //默认路径
	}
	caCert, err := os.ReadFile(caFile) //读取CA证书
	if err != nil {
		return nil, apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to read tracing CA certificate",
			http.StatusInternalServerError,
		)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to parse tracing CA certificate")
	}

	tlsConfig := &tls.Config{
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
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return tlsConfig, nil
}

func (m *Manager) Provider() trace.TracerProvider {
	return m.provider
}
func (m *Manager) Tracer() trace.Tracer {
	return m.tracer
}

// 关闭tracer
func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil || m.shutdown == nil {
		return nil
	}
	return m.shutdown(ctx)
}
