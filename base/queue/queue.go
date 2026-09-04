package queue

type node[T any] struct {
	value T
	next  *node[T]
}

type queue[T any] struct {
	head *node[T]
	tail *node[T]
	size int
}

func (q *queue[T]) IsEmpty() bool {
	return q.size == 0
}
func (q *queue[T]) Size() int {
	return q.size
}

func (q *queue[T]) Push(value T) {
	newNode := &node[T]{value: value}

	if q.tail != nil {
		q.tail.next = newNode
	}
	q.tail = newNode

	// If the queue was empty, head should point to the new node
	if q.head == nil {
		q.head = newNode
	}

	q.size++
}

func (q *queue[T]) Pop() T {
	if q.IsEmpty() {
		panic("tried to pop from empty queue")
	}

	value := q.head.value
	q.head = q.head.next
	q.size--

	// If the queue becomes empty, reset the tail to nil
	if q.head == nil {
		q.tail = nil
	}

	return value
}
func (q *queue[T]) Peek() *T {
	if q.IsEmpty() {
		panic("tried to peek at empty queue")
	}
	return &q.head.value
}
