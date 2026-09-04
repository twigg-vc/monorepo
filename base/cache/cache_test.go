package cache

import (
	"reflect"
	"testing"
)

type MyStruct struct {
	A    string
	Nums []int
}

func TestNoEviction(t *testing.T) {
	c := New[int, MyStruct](3)
	if c.Size() != 0 {
		t.Fatal("size should be 0")
	}

	c.Put(0, MyStruct{"0", []int{0, 0, 0}})
	if c.Size() != 1 {
		t.Fatal("size should be 1")
	}
	c.Put(1, MyStruct{"1", []int{1, 1, 1}})
	if c.Size() != 2 {
		t.Fatal("size should be 2")
	}
	c.Put(2, MyStruct{"2", []int{2, 2, 2}})
	if c.Size() != 3 {
		t.Fatal("size should be 3")
	}

	s, found := c.Get(1)
	if !found {
		t.Fatal("should find 1")
	}
	if !reflect.DeepEqual(s, MyStruct{"1", []int{1, 1, 1}}) {
		t.Fatal("wrong 1 val")
	}

	s, found = c.Get(0)
	if !found {
		t.Fatal("should find 0")
	}
	if !reflect.DeepEqual(s, MyStruct{"0", []int{0, 0, 0}}) {
		t.Fatal("wrong 0 val")
	}

	s, found = c.Get(2)
	if !found {
		t.Fatal("should find 2")
	}
	if !reflect.DeepEqual(s, MyStruct{"2", []int{2, 2, 2}}) {
		t.Fatal("wrong 2 val")
	}
}

func TestEvictOldest(t *testing.T) {
	c := New[int, MyStruct](3)

	c.Put(0, MyStruct{"0", []int{0, 0, 0}})
	c.Put(1, MyStruct{"1", []int{1, 1, 1}})
	c.Put(2, MyStruct{"2", []int{2, 2, 2}})
	c.Put(3, MyStruct{"3", []int{3, 3, 3}})
	if c.Size() != 3 {
		t.Fatal("size should still be 3")
	}

	_, found := c.Get(0)
	if found {
		t.Fatal("o should be evicted")
	}

	s, found := c.Get(1)
	if !found {
		t.Fatal("should find 1")
	}
	if !reflect.DeepEqual(s, MyStruct{"1", []int{1, 1, 1}}) {
		t.Fatal("wrong 1 val")
	}

	s, found = c.Get(2)
	if !found {
		t.Fatal("should find 2")
	}
	if !reflect.DeepEqual(s, MyStruct{"2", []int{2, 2, 2}}) {
		t.Fatal("wrong 2 val")
	}

	s, found = c.Get(3)
	if !found {
		t.Fatal("should find 3")
	}
	if !reflect.DeepEqual(s, MyStruct{"3", []int{3, 3, 3}}) {
		t.Fatal("wrong 3 val")
	}
}

func TestDoesntEvictUsed(t *testing.T) {
	c := New[int, MyStruct](3)

	c.Put(0, MyStruct{"0", []int{0, 0, 0}})
	c.Put(1, MyStruct{"1", []int{1, 1, 1}})
	c.Put(2, MyStruct{"2", []int{2, 2, 2}})
	// Read 0 once so that it's not evicted.
	// 1 should be evicted
	c.Get(0)
	c.Put(3, MyStruct{"3", []int{3, 3, 3}})
	if c.Size() != 3 {
		t.Fatal("size should still be 3")
	}

	s, found := c.Get(0)
	if !found {
		t.Fatal("should find 0")
	}
	if !reflect.DeepEqual(s, MyStruct{"0", []int{0, 0, 0}}) {
		t.Fatal("wrong 0 val")
	}

	_, found = c.Get(1)
	if found {
		t.Fatal("1 should be evicted")
	}

	s, found = c.Get(2)
	if !found {
		t.Fatal("should find 2")
	}
	if !reflect.DeepEqual(s, MyStruct{"2", []int{2, 2, 2}}) {
		t.Fatal("wrong 2 val")
	}

	s, found = c.Get(3)
	if !found {
		t.Fatal("should find 3")
	}
	if !reflect.DeepEqual(s, MyStruct{"3", []int{3, 3, 3}}) {
		t.Fatal("wrong 3 val")
	}
}

func TestRemove(t *testing.T) {

	c := New[int, MyStruct](10)

	c.Put(0, MyStruct{"0", []int{0, 0, 0}})
	c.Put(1, MyStruct{"1", []int{1, 1, 1}})
	c.Put(2, MyStruct{"2", []int{2, 2, 2}})
	if c.Size() != 3 {
		t.Fatal("size should still be 3")
	}

	c.Remove(1)
	if c.Size() != 2 {
		t.Fatal("size should be 2 after remove")
	}

	// Should not find 1
	_, found := c.Get(1)
	if found {
		t.Fatal("should not find 1 after remove")
	}

	// Remaining elements
	s, found := c.Get(0)
	if !found {
		t.Fatal("should find 0")
	}
	if !reflect.DeepEqual(s, MyStruct{"0", []int{0, 0, 0}}) {
		t.Fatal("wrong 0 val")
	}

	s, found = c.Get(2)
	if !found {
		t.Fatal("should find 2")
	}
	if !reflect.DeepEqual(s, MyStruct{"2", []int{2, 2, 2}}) {
		t.Fatal("wrong 2 val")
	}

	// Removing a missing key should not panic
	c.Remove(9)
	if c.Size() != 2 {
		t.Fatal("size should remain 2 after removing non-existent key")
	}
}

func TestRemoveAll(t *testing.T) {

	c := New[int, MyStruct](10)

	c.Put(0, MyStruct{"0", []int{0, 0, 0}})
	c.Put(1, MyStruct{"1", []int{1, 1, 1}})
	c.Put(2, MyStruct{"2", []int{2, 2, 2}})
	if c.Size() != 3 {
		t.Fatal("size should still be 3")
	}
	c.Remove(0)
	c.Remove(1)
	c.Remove(2)
	c.Remove(2)
	if c.Size() != 0 {
		t.Fatal("size should be 0")
	}
}

func TestAddRemoveCallback(t *testing.T) {

	c := New[int, MyStruct](10)

	cb1CallCount := 0
	cb1 := func(k int) {
		if k != 1 {
			t.Fatal("cb1 called from wrong key")
		}
		cb1CallCount += 1
	}
	cb2CallCount := 0
	cb2 := func(k int) {
		if k != 2 {
			t.Fatal("cb2 called from wrong key")
		}
		cb2CallCount += 1
	}
	c.Put(0, MyStruct{"0", []int{0, 0, 0}})
	c.Put(1, MyStruct{"1", []int{1, 1, 1}})
	c.Put(2, MyStruct{"2", []int{2, 2, 2}})
	if c.Size() != 3 {
		t.Fatal("size should still be 3")
	}
	c.AddOnRemoveCallback(1, cb1)
	c.AddOnRemoveCallback(2, cb2)
	c.AddOnRemoveCallback(2, cb2)
	c.Remove(0)
	if cb1CallCount != 0 || cb2CallCount != 0 {
		t.Fatalf("wrong callback counts: %d %d", cb1CallCount, cb2CallCount)
	}
	c.Remove(1)
	if cb1CallCount != 1 || cb2CallCount != 0 {
		t.Fatalf("wrong callback counts: %d %d", cb1CallCount, cb2CallCount)
	}
	c.Remove(2)
	if cb1CallCount != 1 || cb2CallCount != 2 {
		t.Fatalf("wrong callback counts: %d %d", cb1CallCount, cb2CallCount)
	}
	c.Remove(2)
	if cb1CallCount != 1 || cb2CallCount != 2 {
		t.Fatalf("wrong callback counts: %d %d", cb1CallCount, cb2CallCount)
	}
}