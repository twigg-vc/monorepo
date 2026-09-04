package stack

type node[T any] struct {
	value T
	next  *node[T]
}

type stack[T any] struct {
	top  *node[T]
	size int
}

func (q *stack[T]) IsEmpty() bool {
	return q.size == 0
}
func (q *stack[T]) Size() int {
	return q.size
}

func (q *stack[T]) Push(value T) {
	q.top = &node[T]{value: value, next: q.top}
	q.size++
}

func (q *stack[T]) Pop() T {
	if q.IsEmpty() {
		panic("tried to pop from empty stack")
	}

	value := q.top.value
	q.top = q.top.next
	q.size--
	return value
}

func (q *stack[T]) Peek() *T {
	if q.IsEmpty() {
		panic("tried to peek empty stack")
	}
	return &q.top.value
}
