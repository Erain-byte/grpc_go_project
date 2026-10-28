package middleware

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc/stats"
)

// 服务端
func NewServerStatsHandler() stats.Handler {
	return otelgrpc.NewServerHandler(
		otelgrpc.WithMessageEvents(
			otelgrpc.ReceivedEvents,
			otelgrpc.SentEvents,
		),
	)
}

// 客户端
func NewClientStatsHandler() stats.Handler {
	return otelgrpc.NewClientHandler(
		otelgrpc.WithMessageEvents(
			otelgrpc.ReceivedEvents,
			otelgrpc.SentEvents,
		),
	)
}
