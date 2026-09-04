package queue

import "testing"

func Test(t *testing.T) {

	q := New[int]()

	if !q.IsEmpty() {
		t.Fatal("q should be empty")
	}

	q.Push(1)
	if n := q.Peek(); *n != 1 {
		t.Fatal("wrong first peek")
	}
	if n := q.Pop(); n != 1 {
		t.Fatal("wrong first pop")
	}
	q.Push(2)
	q.Push(3)
	q.Push(4)
	if q.Size() != 3 {
		t.Fatal("wrong size")
	}
	if n := q.Peek(); *n != 2 {
		t.Fatal("wrong second peek")
	}
	if n := q.Pop(); n != 2 {
		t.Fatal("wrong second pop")
	}
	if n := q.Peek(); *n != 3 {
		t.Fatal("wrong third peek")
	}
	if n := q.Pop(); n != 3 {
		t.Fatal("wrong third pop")
	}
	if n := q.Pop(); n != 4 {
		t.Fatal("wrong fourth pop")
	}
	if !q.IsEmpty() {
		t.Fatal("queue should be empty")
	}

	// Check panic for empty pop
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("pop from empty should panic")
		}
	}()
	q.Pop()

}
