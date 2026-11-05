package consul

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"web3/pkg/apperror"

	"github.com/hashicorp/consul/api"
)

func (r *ConsulRegistry) queryGRPCService(ctx context.Context, serviceName string, waitIndex uint64) ([]*api.ServiceEntry, uint64, error) {
	if ctx == nil {
		return nil, waitIndex, apperror.InvalidArgument("query Consul: context is nil")
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil, waitIndex, apperror.InvalidArgument("query Consul: service name is empty")
	}
	options := (&api.QueryOptions{WaitIndex: waitIndex, WaitTime: 20 * time.Second}).WithContext(ctx)
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

func (r *ConsulRegistry) WatchGRPCService(ctx context.Context, serviceName string, onUpdate func([]*api.ServiceEntry) error) error {
	if onUpdate == nil {
		return apperror.InvalidArgument("watch Consul: update callback is nil")
	}
	var lastIndex uint64
	for {
		entries, nextIndex, err := r.queryGRPCService(ctx, serviceName, lastIndex)
		if err != nil {
			return err
		}
		if nextIndex < lastIndex {
			lastIndex = 0
			continue
		}
		if nextIndex == 0 {
			nextIndex = 1
		}
		if err := onUpdate(entries); err != nil {
			return err
		}
		lastIndex = nextIndex
	}
}
