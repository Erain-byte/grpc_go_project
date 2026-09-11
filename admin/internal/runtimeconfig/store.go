package runtimeconfig

import (
	"fmt"
	"sync/atomic"
)

type Store struct {
	current atomic.Pointer[Snapshot]
}

func NewStore(init *Snapshot) (*Store, error) {
	if init == nil {
		return nil, fmt.Errorf("init runtime snapshot is nil")
	}
	store := &Store{}
	store.current.Store(init)
	return store, nil
}
func (s *Store) Load() *Snapshot {
	if s == nil {
		return nil
	}
	return s.current.Load()
}

func (s *Store) Store(snapshot *Snapshot) error {
	if s == nil {
		return fmt.Errorf("store is nil")
	}
	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}
	s.current.Store(snapshot)
	return nil

}
