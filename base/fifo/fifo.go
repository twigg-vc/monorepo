package fifo

import (
	"container/list"
)

// entry stores the value and its position in the list
type entry[K comparable, V any] struct {
	key  K
	val  V
	elem *list.Element
}

// cache implements FIFO eviction
type cache[K comparable, V any] struct {
	capacity int
	ll       *list.List
	items    map[K]*entry[K, V]
}

func newCache[K comparable, V any](capacity int) Cache[K, V] {
	if capacity <= 0 {
		panic("capacity must be > 0")
	}
	return &cache[K, V]{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[K]*entry[K, V], capacity),
	}
}

func (c *cache[K, V]) Get(key K) (V, bool) {
	if e, ok := c.items[key]; ok {
		return e.val, true
	}
	var zero V
	return zero, false
}

func (c *cache[K, V]) Put(key K, value V) {
	if e, ok := c.items[key]; ok {
		e.val = value
		return
	}

	// Add new entry
	e := &entry[K, V]{key: key, val: value}
	e.elem = c.ll.PushBack(e) // FIFO: oldest at front, newest at back
	c.items[key] = e

	// Evict if needed
	if len(c.items) > c.capacity {
		oldest := c.ll.Front()
		if oldest != nil {
			ev := oldest.Value.(*entry[K, V])
			delete(c.items, ev.key)
			c.ll.Remove(oldest)
		}
	}
}

func (c *cache[K, V]) Size() int {
	return len(c.items)
}
