package consul

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

func (c *ConsulRegistry) GetKv(ctx context.Context, key string) ([]byte, error) {
	if c == nil || c.client == nil {
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

	pair, _, err := c.client.KV().Get(key, queryOptions)
	if err != nil {
		return nil, fmt.Errorf("read Consul KV: %w", err)
	}
	if pair == nil || len(pair.Value) == 0 {
		return nil, fmt.Errorf("read Consul KV: key not found")
	}
	return pair.Value, nil
}

func (c *ConsulRegistry) WatchKv(ctx context.Context, key string, waitIndex uint64) ([]byte, uint64, error) {
	if c == nil || c.client == nil {
		return nil, 0, fmt.Errorf("watch Consul KV: client is nil")
	}
	if ctx == nil {
		return nil, 0, fmt.Errorf("watch Consul KV: context is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, 0, fmt.Errorf("watch Consul KV: key is empty")
	}
	queryOptions := (&api.QueryOptions{WaitIndex: waitIndex, WaitTime: 5 * time.Minute}).WithContext(ctx)

	pair, meta, err := c.client.KV().Get(key, queryOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("watch Consul KV: %w", err)

	}
	if pair == nil || len(pair.Value) == 0 {
		return nil, 0, fmt.Errorf("watch Consul KV: key not found")
	}
	index := pair.ModifyIndex
	if meta != nil {
		index = meta.LastIndex
	}
	return append([]byte(nil), pair.Value...), index, nil
}
