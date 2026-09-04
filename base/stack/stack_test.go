package stack

import "testing"

func Test(t *testing.T) {

	st := New[int]()

	if !st.IsEmpty() {
		t.Fatal("q should be empty")
	}

	st.Push(1)
	if n := st.Peek(); *n != 1 {
		t.Fatal("wrong first peek")
	}
	if n := st.Pop(); n != 1 {
		t.Fatal("wrong first pop")
	}
	st.Push(2)
	st.Push(3)
	st.Push(4)
	if st.Size() != 3 {
		t.Fatal("wrong size")
	}
	if n := st.Peek(); *n != 4 {
		t.Fatal("wrong second peek")
	}
	if n := st.Pop(); n != 4 {
		t.Fatal("wrong second pop")
	}
	if n := st.Pop(); n != 3 {
		t.Fatal("wrong third pop")
	}
	if n := st.Pop(); n != 2 {
		t.Fatal("wrong fourth pop")
	}
	if !st.IsEmpty() {
		t.Fatal("queue should be empty")
	}

	// Check panic for empty pop
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("pop from empty should panic")
		}
	}()
	st.Pop()

}
