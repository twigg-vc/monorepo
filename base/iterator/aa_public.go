package iterator

import (
	"errors"
)

// Iterator of type T
type I[T any] interface {
	Get() (T, error)
	Next() bool
	Err() error
}

// Helper to get the n first entries from an iterator
func GetFirstN[T any](n int, it I[T]) ([]T, error) {
	identityF := func(i T) (T, error) {
		return i, nil
	}
	return GetFirstNWithMapFunc(n, it, identityF)
}

// Helper to get the n first entries from an iterator with some function
// that maps (i.e. "converts") each item into an output
func GetFirstNWithMapFunc[IterT any, OutputT any](n int, it I[IterT], mapFunc func(i IterT) (OutputT, error)) ([]OutputT, error) {
	mapper := fMapper[IterT, OutputT]{
		mapFunc: mapFunc,
	}
	return GetFirstNWithMapper(n, it, mapper)
}

// Helper to get the n first entries from an iterator with some function
// that converts each item into an output
func GetFirstNWithMapper[IterT any, OutputT any](n int, it I[IterT], m Mapper[IterT, OutputT]) ([]OutputT, error) {
	if n < 0 {
		panic("used GetFirstN* with n < 0")
	}
	entries := make([]OutputT, 0, n)
	for it.Next() {
		if len(entries) == n {
			break
		}
		iterEntry, err := it.Get()
		if err != nil {
			return nil, err
		}
		outputEntry, err := m.Map(iterEntry)
		if err != nil {
			return nil, err
		}
		entries = append(entries, outputEntry)
	}
	err := it.Err()
	if err != nil {
		return nil, err
	}
	return entries, nil
}

type Mapper[IterT any, OutputT any] interface {
	Map(i IterT) (OutputT, error)
}

// Must be created with `NewIterFromSlice`
// Helper to create an iterator from a slice (usually for tests)
type SliceIter[T any] struct {
	Entries []T
	i       int
}

func NewIterFromSlice[T any](entries []T) *SliceIter[T] {
	return &SliceIter[T]{Entries: entries, i: -1}
}

func (si SliceIter[T]) Get() (t T, err error) {
	if si.i >= len(si.Entries) {
		err = errors.New("done iterating")
		return
	}
	return si.Entries[si.i], nil
}
func (si *SliceIter[T]) Next() bool {
	si.i += 1
	return si.i < len(si.Entries)
}
func (si SliceIter[T]) Err() error {
	return nil
}
