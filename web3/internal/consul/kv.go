package consul

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

func (r *ConsulRegistry) GetKV(ctx context.Context, key string) ([]byte, error) {
	value, _, err := r.watchKV(ctx, key, 0, 0)
	return value, err
}

func (r *ConsulRegistry) WatchKV(ctx context.Context, key string, waitIndex uint64) ([]byte, uint64, error) {
	return r.watchKV(ctx, key, waitIndex, 5*time.Minute)
}

func (r *ConsulRegistry) watchKV(ctx context.Context, key string, waitIndex uint64, waitTime time.Duration) ([]byte, uint64, error) {
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
	pair, meta, err := r.client.KV().Get(key, (&api.QueryOptions{WaitIndex: waitIndex, WaitTime: waitTime}).WithContext(ctx))
	if err != nil {
		return nil, waitIndex, fmt.Errorf("read Consul KV: %w", err)
	}
	if pair == nil || len(pair.Value) == 0 {
		return nil, waitIndex, fmt.Errorf("read Consul KV: key %q not found", key)
	}
	index := pair.ModifyIndex
	if meta != nil {
		index = meta.LastIndex
	}
	return append([]byte(nil), pair.Value...), index, nil
}
