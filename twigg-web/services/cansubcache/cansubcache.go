package cansubcache

import (
	"fmt"
	"math"
	"monorepo/base/cache"
	"monorepo/twigg/server"
	"sync"
	"unsafe"
)

func newCanSubCache(cacheCapacity int) *canSubCache {
	keySizeInBytes := int64(len(cacheKey(math.MaxUint64, math.MaxUint64, math.MaxUint64)))
	var v cacheVal
	cacheValSizeInBytes := int64(unsafe.Sizeof(v))
	return &canSubCache{
		mu:                    sync.RWMutex{},
		c:                     cache.New[string, cacheVal](cacheCapacity),
		memUsageEstimateBytes: (keySizeInBytes + cacheValSizeInBytes) * int64(cacheCapacity),
	}
}

type canSubCache struct {
	mu                    sync.RWMutex
	c                     cache.LRU[string, cacheVal]
	memUsageEstimateBytes int64
}

type cacheVal struct {
	canSub        bool
	cantSubReason server.CantSubmitReason
}

func cacheKey(commitId, commitVersion, topCommitId uint64) string {
	return fmt.Sprintf("%d-%d-%d", commitId, commitVersion, topCommitId)
}

func (c *canSubCache) GetCanSubmit(commitId, commitVersion, topCommitId uint64) (canSubmit bool, cantSubReason server.CantSubmitReason, cacheFound bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var val cacheVal
	val, cacheFound = c.c.Get(cacheKey(commitId, commitVersion, topCommitId))
	canSubmit = val.canSub
	cantSubReason = val.cantSubReason
	return
}
func (c *canSubCache) PutCanSubmit(commitId, commitVersion, topCommitId uint64, canSubmit bool, cantSubReason server.CantSubmitReason) {
	c.mu.Lock()
	defer c.mu.Unlock()
	val := cacheVal{
		canSub:        canSubmit,
		cantSubReason: cantSubReason,
	}
	c.c.Put(cacheKey(commitId, commitVersion, topCommitId), val)
}
func (c *canSubCache) GetMaxMemUsageEstimate() int64 {
	return c.memUsageEstimateBytes
}
