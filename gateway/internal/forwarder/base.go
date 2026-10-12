package forwarder

import (
	"context"

	grpcclient "gateway/internal/grpc"
	"gateway/pkg/apperror"

	"google.golang.org/grpc"
)

// BaseForwarder leaves service discovery, connection reuse, and protobuf
// client construction to ClientManager.
type BaseForwarder[T any] struct {
	serviceName string

	forwardClient *grpcclient.ForwardClient[T]
}

func NewBaseForwarder[T any](clientManager *grpcclient.ClientManager, serviceName string, factory func(grpc.ClientConnInterface) T) *BaseForwarder[T] {
	return &BaseForwarder[T]{
		//clientManager: clientManager,
		serviceName: serviceName,
		//factory: factory,
		forwardClient: grpcclient.NewForwardClient[T](factory, clientManager),
	}
}

func (b *BaseForwarder[T]) GetClient(ctx context.Context) (T, error) {
	var zero T
	if b == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("forwarder is nil"))
	}
	//client, err := grpcclient.CreateClient(ctx, b.clientManager, b.serviceName, b.factory)
	client, err := b.forwardClient.Forward(ctx, b.serviceName)
	if err != nil {
		return zero, apperror.ToGRPC(err)
	}
	return client, nil
}
