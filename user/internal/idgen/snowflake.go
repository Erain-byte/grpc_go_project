// Package idgen provides process-wide unique ID generators.
package idgen

import (
	"fmt"
	"sync"
	"time"
)

const (
	nodeBits     = 10
	sequenceBits = 12

	maxNodeID    = int64(1<<nodeBits - 1)
	sequenceMask = int64(1<<sequenceBits - 1)
	nodeShift    = sequenceBits
	timeShift    = nodeBits + sequenceBits

	// 2026-01-01T00:00:00Z. A custom epoch leaves roughly 69 years of
	// timestamp capacity while keeping generated IDs positive.
	defaultEpochMillis = int64(1767225600000)
)

var _ Generator = (*Snowflake)(nil)

// Generator is the small interface consumed by business services.
type Generator interface {
	NextID() (int64, error)
}

// Snowflake generates IDs using timestamp, node ID, and per-millisecond
// sequence fields. One instance is safe for concurrent use.
type Snowflake struct {
	mu sync.Mutex

	nodeID        int64
	epochMillis   int64
	lastMillis    int64
	sequence      int64
	now           func() time.Time
	waitNextMilli func(lastMillis int64) int64
}

// NewSnowflake creates a generator for nodeID. Every simultaneously running
// User service instance must use a different node ID.
func NewSnowflake(nodeID int64) (*Snowflake, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("snowflake node ID %d is outside 0..%d", nodeID, maxNodeID)
	}

	generator := &Snowflake{
		nodeID:      nodeID,
		epochMillis: defaultEpochMillis,
		lastMillis:  -1,
		now:         time.Now,
	}
	generator.waitNextMilli = generator.waitUntilNextMilli
	return generator, nil
}

// NextID returns a positive, process-wide unique ID. It returns an error when
// the system clock moves backwards instead of risking duplicate IDs.
func (s *Snowflake) NextID() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nowMillis := s.now().UnixMilli()
	if nowMillis < s.epochMillis {
		return 0, fmt.Errorf("system time %d is earlier than snowflake epoch %d", nowMillis, s.epochMillis)
	}
	if nowMillis < s.lastMillis {
		return 0, fmt.Errorf("system clock moved backwards by %dms", s.lastMillis-nowMillis)
	}

	if nowMillis == s.lastMillis {
		s.sequence = (s.sequence + 1) & sequenceMask
		if s.sequence == 0 {
			nowMillis = s.waitNextMilli(s.lastMillis)
		}
	} else {
		s.sequence = 0
	}

	timestamp := nowMillis - s.epochMillis
	if timestamp >= 1<<41 {
		return 0, fmt.Errorf("snowflake timestamp capacity exhausted")
	}

	s.lastMillis = nowMillis
	return (timestamp << timeShift) | (s.nodeID << nodeShift) | s.sequence, nil
}

func (s *Snowflake) waitUntilNextMilli(lastMillis int64) int64 {
	for {
		nowMillis := s.now().UnixMilli()
		if nowMillis > lastMillis {
			return nowMillis
		}
		time.Sleep(100 * time.Microsecond)
	}
}
