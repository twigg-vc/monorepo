package stack

// LIFO queue that provides O(1) Push, Pop and IsEmpty.
type Stack[T any] interface {
	// Checks if it's empty
	IsEmpty() bool
	// Returns the number of entries
	Size() int
	// Push to the end
	Push(t T)
	// Shows the top val without removing it
	// Panics if empty
	Peek() *T
	// Panics if empty
	Pop() T
}

func New[T any]() Stack[T] {
	return &stack[T]{}
}
