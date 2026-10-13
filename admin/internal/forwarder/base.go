package forwarder

import (
	"context"

	grpcclient "admin/internal/grpcclient"
	apperror "admin/pkg/apperorr"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// BaseForwarder leaves service discovery, connection reuse, and protobuf
// client construction to ClientManager.
type BaseForwarder[T any] struct {
	serviceName string

	forwardClient *grpcclient.ForwardClient[T]
}

// OutgoingContext propagates the bearer token so each downstream service can
// validate the original Keycloak access token independently.
func OutgoingContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	values := md.Get("authorization")
	if len(values) != 1 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", values[0])
}

func NewBaseForwarder[T any](clientManager *grpcclient.ClientManager, serviceName string, factory func(grpc.ClientConnInterface) T) *BaseForwarder[T] {
	return &BaseForwarder[T]{
		//clientManager: clientManager,
		serviceName: serviceName,
		//factory: factory,
		forwardClient: grpcclient.NewForwardClient[T](factory, clientManager),
	}
}

func (f *BaseForwarder[T]) Forward(ctx context.Context) (T, error) {
	var zero T
	if f == nil || f.forwardClient == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("forwarder is nil"))
	}

	client, err := f.forwardClient.ForwardBase(ctx, f.serviceName)
	if err != nil {
		return zero, apperror.ToGRPC(err)
	}
	return client, nil
}
