package fifo

import (
	"testing"
)

func TestFIFOCache(t *testing.T) {
	c := New[string, int](3)

	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	if c.Size() != 3 {
		t.Fatalf("expected size 3, got %d", c.Size())
	}

	// Check existing keys
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("expected a=1, got %v, ok=%v", v, ok)
	}

	// Insert new element, should evict "a" (FIFO)
	c.Put("d", 4)

	if c.Size() != 3 {
		t.Fatalf("expected size 3 after eviction, got %d", c.Size())
	}

	if _, ok := c.Get("a"); ok {
		t.Fatalf("expected a to be evicted")
	}

	// Existing elements should still be there
	for key, val := range map[string]int{"b": 2, "c": 3, "d": 4} {
		if v, ok := c.Get(key); !ok || v != val {
			t.Fatalf("expected %s=%d, got %v, ok=%v", key, val, v, ok)
		}
	}

	// Update existing key
	c.Put("b", 20)
	if v, ok := c.Get("b"); !ok || v != 20 {
		t.Fatalf("expected b=20 after update, got %v, ok=%v", v, ok)
	}
}
