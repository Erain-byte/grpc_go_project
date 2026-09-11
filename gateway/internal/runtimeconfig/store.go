package runtimeconfig

import (
	"fmt"
	"sync/atomic"
)

// Store 原子保存最后一份有效配置，读取路径不需要加锁。
type Store struct {
	current atomic.Pointer[Snapshot]
}

func NewStore(initial *Snapshot) (*Store, error) {
	if initial == nil {
		return nil, fmt.Errorf("initial runtime snapshot is nil")
	}
	store := &Store{}
	store.current.Store(initial)
	return store, nil
}

func (s *Store) Load() *Snapshot {
	if s == nil {
		return nil
	}
	return s.current.Load()
}

func (s *Store) Store(snapshot *Snapshot) error {
	if s == nil || snapshot == nil {
		return fmt.Errorf("runtime store or snapshot is nil")
	}
	s.current.Store(snapshot)
	return nil
}
