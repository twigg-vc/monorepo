package cache

// Least recently used cache.
// NOT thread safe
type LRU[K comparable, V any] interface {
	Get(key K) (val V, found bool)
	Put(key K, value V)
	Remove(key K)
	Size() int
	// Add a function to be called when an entry is removed
	AddOnRemoveCallback(key K, callback func(key K))
}

func New[K comparable, V any](capacity int) LRU[K, V] {
	return newCache[K, V](capacity)
}