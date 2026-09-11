package consul

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

// GetKV 只负责从 Consul 读取原始配置，不解析任何 Gateway 业务字段。
func (r *ConsulRegistry) GetKV(ctx context.Context, key string) ([]byte, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("read Consul KV: client is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("read Consul KV: context is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("read Consul KV: key is empty")
	}

	queryOptions := (&api.QueryOptions{}).WithContext(ctx)
	pair, _, err := r.client.KV().Get(key, queryOptions)
	if err != nil {
		return nil, fmt.Errorf("read Consul KV %q: %w", key, err)
	}
	if pair == nil || len(pair.Value) == 0 {
		return nil, fmt.Errorf("Consul KV %q does not exist or is empty", key)
	}
	return pair.Value, nil
}

// Watch KV 只负责从 Consul 读取原始配置，不解析任何 Gateway 业务字段。
func (r *ConsulRegistry) WatchKV(ctx context.Context, key string, waitIndex uint64) (value []byte, modifyIndex uint64, returnErr error) {
	if r == nil || r.client == nil {
		return nil, waitIndex, fmt.Errorf("read Consul KV: client is nil")
	}
	if ctx == nil {
		return nil, waitIndex, fmt.Errorf("read Consul KV: context is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, waitIndex, fmt.Errorf("read Consul KV: key is empty")
	}
	query := (&api.QueryOptions{
		WaitIndex: waitIndex,
		WaitTime:  5 * time.Minute,
	}).WithContext(ctx)
	pair, meta, err := r.client.KV().Get(key, query)
	if err != nil {
		return nil, waitIndex, fmt.Errorf("read Consul KV %q: %w", key, err)
	}
	if pair == nil || len(pair.Value) == 0 {
		return nil, waitIndex, fmt.Errorf("Consul KV %q does not exist or is empty", key)
	}
	index := pair.ModifyIndex

	if meta != nil && meta.LastIndex > index {
		index = meta.LastIndex
	}
	return append([]byte(nil), pair.Value...), index, nil
}
