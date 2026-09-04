package cansubcache

import (
	"monorepo/twigg/server"
)

// "Best-effort" cache for storing when commits can/cant' be submitted
type CanSubCache struct {
	c *canSubCache
}

func New(cacheCapacity int) CanSubCache {
	return CanSubCache{
		c: newCanSubCache(cacheCapacity),
	}
}

func (c CanSubCache) GetCanSubmit(commitId, commitVersion, topCommitId uint64) (canSubmit bool, cantSubReason server.CantSubmitReason, cacheFound bool) {
	return c.c.GetCanSubmit(commitId, commitVersion, topCommitId)
}
func (c CanSubCache) PutCanSubmit(commitId, commitVersion, topCommitId uint64, canSubmit bool, cantSubReason server.CantSubmitReason) {
	c.c.PutCanSubmit(commitId, commitVersion, topCommitId, canSubmit, cantSubReason)
}
func (c CanSubCache) GetCacheSize() int {
	return c.c.c.Size()
}

// Returns a rough estimate of how much memory (bytes) the cache will use.
func (c CanSubCache) GetMaxMemUsageEstimate() int64 {
	return c.c.GetMaxMemUsageEstimate()
}