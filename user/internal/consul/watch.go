package consul

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"user/pkg/apperror"

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
	// 查询健康的服务实例，最多等待 20 秒
	options := (&api.QueryOptions{
		WaitIndex: waitIndex,
		WaitTime:  20 * time.Second,
	}).WithContext(ctx)

	entries, meta, err := r.client.Health().Service(serviceName, ProtocolGRPC, true, options)
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
func (r *ConsulRegistry) WatchGRPCService(ctx context.Context, serviceName string, callback func([]*api.ServiceEntry) error) error {
	if ctx == nil {
		return apperror.InvalidArgument("watch consul: context is nil")
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return apperror.InvalidArgument("watch consul: service name is empty")
	}
	//最后一次的索引
	var lastIndex uint64
	for {
		entries, newIndex, err := r.queryGRPCService(ctx, serviceName, lastIndex) // 查询健康的服务实例
		if err != nil {
			return err
		}
		//判断当前index是否小于上一次的index，如果是则继续查询
		if newIndex < lastIndex {
			lastIndex = 0
			continue
		}
		if newIndex == 0 {
			newIndex = 1
		}
		if err := callback(entries); err != nil {
			return err
		}
		lastIndex = newIndex
	}
}
