package queue

// FIFO queue that provides O(1) Push, Pop and IsEmpty.
type Queue[T any] interface {
	// Checks if it's empty
	IsEmpty() bool
	// Returns the number of entries
	Size() int
	// Push to the end
	Push(t T)
	// Panics if queue is empty
	Pop() T
	// Look and the next without popping it
	Peek() *T
}

func New[T any]() Queue[T] {
	return &queue[T]{}
}
