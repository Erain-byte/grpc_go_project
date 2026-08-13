package middleware

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc/stats"
)

// grpc的handler
func NewServerStatsHandler() stats.Handler { //创建一个grpc的handler
	return otelgrpc.NewServerHandler(

		//otelgrpc.WithTracerProvider(tracer.Manager.Provider()),                   //设置grpc的tracerProvider
		otelgrpc.WithMessageEvents(otelgrpc.ReceivedEvents, otelgrpc.SentEvents), //设置grpc的消息事件
	) //使用otelgrpc的NewServerHandler方法

}

// grpc的handler
func NewClientStatsHandler() stats.Handler {
	return otelgrpc.NewClientHandler(
		//otelgrpc.WithTracerProvider(tracer.TracerProvider),                       //设置grpc的tracerProvider
		otelgrpc.WithMessageEvents(otelgrpc.ReceivedEvents, otelgrpc.SentEvents), //设置grpc的消息事件
	)
}
