package cache

import "container/list"

type entry[K comparable, V any] struct {
	key   K
	value V
}

type lruCache[K comparable, V any] struct {
	capacity  int
	ll        *list.List
	cache     map[K]*list.Element
	onRemoves map[K]*[]func(key K)
}

func newCache[K comparable, V any](capacity int) *lruCache[K, V] {
	if capacity <= 0 {
		panic("lru: capacity must be greater than 0")
	}
	return &lruCache[K, V]{
		capacity:  capacity,
		ll:        list.New(),
		cache:     make(map[K]*list.Element, capacity),
		onRemoves: map[K]*[]func(key K){},
	}
}

// Get returns the value and moves the entry to the front (most recently used).
func (c *lruCache[K, V]) Get(key K) (val V, found bool) {
	if elem, ok := c.cache[key]; ok {
		c.ll.MoveToFront(elem)
		return elem.Value.(*entry[K, V]).value, true
	}
	return val, false
}

// Put inserts or updates the value and marks it as most recently used.
func (c *lruCache[K, V]) Put(key K, value V) {
	if elem, ok := c.cache[key]; ok {
		// Update existing entry and move to front
		elem.Value.(*entry[K, V]).value = value
		c.ll.MoveToFront(elem)
		return
	}

	// Insert new entry
	e := &entry[K, V]{key, value}
	elem := c.ll.PushFront(e)
	c.cache[key] = elem

	// Evict least recently used if over capacity
	if c.ll.Len() > c.capacity {
		c.RemoveOldest()
	}
}

// Remove deletes an entry if present.
func (c *lruCache[K, V]) Remove(key K) {
	if elem, ok := c.cache[key]; ok {
		c.removeElement(elem)
	}
}

// Size returns the number of items currently stored.
func (c *lruCache[K, V]) Size() int {
	return c.ll.Len()
}
func (c *lruCache[K, V]) Capacity() int {
	return c.capacity
}

func (c *lruCache[K, V]) AddOnRemoveCallback(key K, onRemove func(key K)) {
	callbacks, ok := c.onRemoves[key]
	if !ok {
		cbs := make([]func(key K), 0)
		c.onRemoves[key] = &cbs
		callbacks = &cbs
	}
	*callbacks = append(*callbacks, onRemove)
}

// removeOldest removes the least recently used element.
func (c *lruCache[K, V]) RemoveOldest() *K {
	elem := c.ll.Back()
	if elem != nil {
		k := c.removeElement(elem)
		return &k
	}
	return nil
}

// removeElement removes an element from both the list and the map.
func (c *lruCache[K, V]) removeElement(elem *list.Element) K {
	ent := elem.Value.(*entry[K, V])
	k := ent.key
	delete(c.cache, ent.key)
	c.ll.Remove(elem)
	callbacks, ok := c.onRemoves[k]
	if ok {
		for _, cb := range *callbacks {
			cb(k)
		}
		delete(c.onRemoves, k)
	}
	return k
}