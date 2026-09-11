package runtimeconfig

import (
	"sync"
	"testing"
)

func TestStoreConcurrentLoadAndStore(t *testing.T) {
	store, err := NewStore(&Snapshot{Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(offset int) {
			defer group.Done()
			for index := 1; index <= 100; index++ {
				if index%2 == 0 {
					_ = store.Store(&Snapshot{Version: offset*100 + index})
				} else if store.Load() == nil {
					t.Error("Store.Load() returned nil")
				}
			}
		}(worker)
	}
	group.Wait()
}
