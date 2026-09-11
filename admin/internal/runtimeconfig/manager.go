package runtimeconfig

import (
	"admin/internal/config"
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Manager struct {
	key     string
	store   *Store
	logger  *zap.Logger
	watcher KVWatcher
}

type KVWatcher interface {
	WatchKv(ctx context.Context, key string, waitIndex uint64) ([]byte, uint64, error)
}

func NewManager(key string, store *Store, logger *zap.Logger, watcher KVWatcher) (*Manager, error) {
	key = strings.TrimSpace(key)
	if key == "" || watcher == nil || store == nil {
		return nil, fmt.Errorf("runtime config manager dependencies are incomplete")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		key:     key,
		store:   store,
		logger:  logger,
		watcher: watcher,
	}, nil
}

// Run 监听配置变化。运行中的新配置非法或 Consul 暂时不可用时，保留最后有效快照。
func (m *Manager) Run(ctx context.Context) {
	var waitIndex uint64
	retryDelay := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		data, modifyIndex, err := m.watcher.WatchKv(ctx, m.key, waitIndex)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.logger.Error("runtime config watch failed", zap.Error(err), zap.Duration("retry_after", retryDelay))
			if !waitRetry(ctx, retryDelay) {
				return
			}
			retryDelay *= 2
			if retryDelay > 30*time.Second {
				retryDelay = 30 * time.Second
			}
			continue
		}
		if modifyIndex <= waitIndex {
			continue
		}
		// 无论配置是否有效都推进 Index，避免非法版本被立即反复读取。
		waitIndex = modifyIndex
		parsed, err := config.ParseRuntimeConfig(data)
		if err != nil {
			m.logger.Error("runtime config validation failed; keeping last valid snapshot", zap.Uint64("modify_index", modifyIndex), zap.Error(err))
			continue
		}
		snapshot, err := BuildSnapshot(parsed)
		if err != nil {
			m.logger.Error("runtime snapshot build failed; keeping last valid snapshot", zap.Uint64("modify_index", modifyIndex), zap.Error(err))
			continue
		}
		if err := m.store.Store(snapshot); err != nil {
			m.logger.Error("runtime snapshot publish failed", zap.Error(err))
			continue
		}
		retryDelay = time.Second
		m.logger.Info("runtime config updated", zap.Int("version", snapshot.Version), zap.Uint64("modify_index", modifyIndex))
	}
}

func waitRetry(ctx context.Context, delay time.Duration) bool {
	time := time.NewTimer(delay)
	defer time.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-time.C:
		return true
	}
}
