package cansubcache

import (
	"monorepo/twigg/server"
	"runtime"
	"testing"
)

func TestSimpleHappyPath(t *testing.T) {
	c := New(10)

	if c.GetCacheSize() != 0 {
		t.Fatalf("c.GetCacheSize(): %d", c.GetCacheSize())
	}

	_, _, cacheFound := c.GetCanSubmit(1, 2, 3)
	if cacheFound {
		t.Fatalf("empty cache got cacheFound")
	}

	c.PutCanSubmit(1, 2, 3, true, server.CantSubmitReasonNone)
	canSub, canSubReason, cacheFound := c.GetCanSubmit(1, 2, 3)
	if !cacheFound {
		t.Fatalf("cache not found after put")
	}
	if canSub != true {
		t.Fatal("got false for canSub after put with canSub=true")
	}
	if canSubReason != server.CantSubmitReasonNone {
		t.Fatalf("expected cantSubReason=%d, got %d", server.CantSubmitReasonNone, canSubReason)
	}
}

func TestEnvictions(t *testing.T) {
	// Use a small cache size to force evictions
	c := New(3)

	c.PutCanSubmit(1, 1, 1, true, server.CantSubmitReasonNone)
	c.PutCanSubmit(2, 2, 2, false, server.CantSubmitWouldCauseRebaseConflict)
	c.PutCanSubmit(3, 3, 3, false, server.CantSubmitWouldCauseRebaseConflict)
	c.PutCanSubmit(4, 4, 4, true, server.CantSubmitReasonNone)

	// One entry will be evicted
	if c.GetCacheSize() != 3 {
		t.Fatalf("c.GetCacheSize(): %d", c.GetCacheSize())
	}

	_, _, cacheFound := c.GetCanSubmit(1, 1, 1)
	if cacheFound {
		t.Fatalf("first entry was not evicted")
	}

	canSub, cantSubReason, cacheFound := c.GetCanSubmit(2, 2, 2)
	if !cacheFound {
		t.Fatalf("cache 2 not found")
	}
	if canSub != false || cantSubReason != server.CantSubmitWouldCauseRebaseConflict {
		t.Fatalf("unexpected val for cache 2: %v, %d", canSub, cantSubReason)
	}

	canSub, cantSubReason, cacheFound = c.GetCanSubmit(3, 3, 3)
	if !cacheFound {
		t.Fatalf("cache 3 not found")
	}
	if canSub != false || cantSubReason != server.CantSubmitWouldCauseRebaseConflict {
		t.Fatalf("unexpected val for cache 3: %v, %d", canSub, cantSubReason)
	}

	canSub, cantSubReason, cacheFound = c.GetCanSubmit(4, 4, 4)
	if !cacheFound {
		t.Fatalf("cache 4 not found")
	}
	if canSub != true || cantSubReason != server.CantSubmitReasonNone {
		t.Fatalf("unexpected val for cache 4: %v, %d", canSub, cantSubReason)
	}
}

func TestSizeEstimate(t *testing.T) {
	const cacheCapacity = 500
	c := New(cacheCapacity)
	estimate := c.GetMaxMemUsageEstimate()
	if estimate < 0 {
		t.Fatalf("got negative mem usage estimate")
	}

	var before runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	// fill cache
	for i := 0; i < cacheCapacity; i++ {
		c.PutCanSubmit(uint64(i), 0, 0, true, server.CantSubmitWouldCauseRebaseConflict)
	}
	if c.GetCacheSize() != cacheCapacity {
		t.Fatalf("cache not full")
	}

	var after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&after)

	gotMemUsage := int64(after.Alloc) - int64(before.Alloc)
	if estimate < 0 {
		t.Fatalf("got negative mem usage estimate")
	}
	// Expect the estimate to be within 2x to 0.5 of the actual measurement
	if estimate > 2*gotMemUsage || estimate < gotMemUsage/2 {
		t.Fatalf("expected mem ~%d got %d", estimate, gotMemUsage)
	}

	runtime.KeepAlive(c) // Necessary for the runtime to not free c
}
