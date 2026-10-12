package consul

import (
	apperror "admin/pkg/apperorr"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

const ProtocolGRPC = "grpc"

// queryGRPCService 使用 Consul Blocking Query 查询健康的 gRPC 服务实例。

func (r *ConsulRegistry) queryGRPCService(ctx context.Context, serviceName string, waitIndex uint64) ([]*api.ServiceEntry, uint64, error) {
	if ctx == nil {
		return nil, waitIndex, apperror.InvalidArgument("query consul: context is nil")
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil, waitIndex, apperror.InvalidArgument("query consul: service name is empty")
	}
	options := (&api.QueryOptions{
		WaitIndex: waitIndex,
		WaitTime:  20 * time.Second,
	}).WithContext(ctx)
	entries, meta, err := r.client.Health().Service(serviceName, protocolGRPC, true, options)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, waitIndex, apperror.Wrap(err, apperror.CodeTimeout, "Consul query timed out", http.StatusGatewayTimeout)

		}
		return nil, waitIndex, apperror.Wrap(err, apperror.CodeUnavailable, "failed to query service from Consul", http.StatusServiceUnavailable)
	}
	if meta == nil {
		return nil, waitIndex, apperror.Unavailable("Consul query returned no metadata")
	}
	return entries, meta.LastIndex, nil
}

// WatchGRPCService 持续监听健康的 gRPC 服务实例，并在地址发生查询更新时回调
func (r *ConsulRegistry) WatchGRPCService(ctx context.Context, serviceName string, onUpdate func([]*api.ServiceEntry) error) error {
	if ctx == nil {
		return apperror.InvalidArgument("watch consul: context is nil")
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return apperror.InvalidArgument("watch consul: service name is empty")
	}
	var lastIndex uint64
	for {
		entries, newIndex, err := r.queryGRPCService(ctx, serviceName, lastIndex)
		if err != nil {
			return err
		}
		if newIndex < lastIndex {
			lastIndex = 0
			continue
		}
		if newIndex == 0 {
			newIndex = 1
		}
		if err := onUpdate(entries); err != nil {
			return err
		}
		lastIndex = newIndex

	}

}
