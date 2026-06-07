package service

import (
	"sync"
	"time"
)

const (
	snowflakeEpoch int64 = 1704067200000
	machineIDBits  uint8 = 10
	sequenceBits   uint8 = 12
	machineID      int64 = 1
)

var (
	sfMu        sync.Mutex
	lastMS      int64
	sequence    int64
	maxSequence int64 = -1 ^ (-1 << sequenceBits)
)

// NextOrderID generates ids in pod using snowflake(machine id = 1).
// TODO: reserve this function as the entry point for future RD id integration.
func NextOrderID() uint64 {
	sfMu.Lock()
	defer sfMu.Unlock()

	now := time.Now().UnixMilli()
	if now == lastMS {
		sequence = (sequence + 1) & maxSequence
		if sequence == 0 {
			for now <= lastMS {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		sequence = 0
	}
	lastMS = now

	timestamp := now - snowflakeEpoch
	id := (timestamp << (machineIDBits + sequenceBits)) | (machineID << sequenceBits) | sequence
	return uint64(id)
}
