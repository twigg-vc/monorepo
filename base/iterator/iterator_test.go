package iterator

import (
	"fmt"
	"reflect"
	"testing"
)

func TestGetFirst3(t *testing.T) {
	it := NewIterFromSlice([]int{0, 1, 2, 3, 4, 5})
	nums, err := GetFirstN(3, it)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nums, []int{0, 1, 2}) {
		t.Fatalf("got: %v", nums)
	}
}

func TestGetFirstZero(t *testing.T) {
	it := NewIterFromSlice([]int{0, 1, 2, 3, 4, 5})
	nums, err := GetFirstN(0, it)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nums, []int{}) {
		t.Fatalf("got: %v", nums)
	}
}

func TestGetFirstAll(t *testing.T) {
	it := NewIterFromSlice([]int{0, 1, 2})
	nums, err := GetFirstN(3, it)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nums, []int{0, 1, 2}) {
		t.Fatalf("got: %v", nums)
	}
}
func TestEmptyIter(t *testing.T) {
	it := NewIterFromSlice([]int{})
	nums, err := GetFirstN(10, it)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nums, []int{}) {
		t.Fatalf("got: %v", nums)
	}
}

func TestNilIter(t *testing.T) {
	it := NewIterFromSlice[int](nil)
	nums, err := GetFirstN(10, it)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nums, []int{}) {
		t.Fatalf("got: %v", nums)
	}
}

func TestGetFirstNWithMapFunc(t *testing.T) {
	it := NewIterFromSlice([]int{0, 1, 2})
	mapFunc := func(x int) (string, error) {
		return fmt.Sprintf("%d", x), nil
	}
	stringVals, err := GetFirstNWithMapFunc(3, it, mapFunc)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stringVals, []string{"0", "1", "2"}) {
		t.Fatalf("got: %v", stringVals)
	}
}

func TestGetFirstNWithMapFunc_MapFuncErrors(t *testing.T) {
	it := NewIterFromSlice([]int{0, 1, 2})
	mapFuncCalls := []int{}
	mapFunc := func(x int) (string, error) {
		mapFuncCalls = append(mapFuncCalls, x)
		if x == 1 {
			return "", fmt.Errorf("BOOM")
		}
		return fmt.Sprintf("%d", x), nil
	}
	// Ensure we can an error back if a mapper call fails
	_, err := GetFirstNWithMapFunc(3, it, mapFunc)
	if err == nil {
		t.Fatalf("mapper error'd but GetFirstNWithMapFunc got nil err")
	}
	// Ensure the iterator stops on error
	if !reflect.DeepEqual(mapFuncCalls, []int{0, 1}) {
		t.Fatalf("got mapFuncCalls: %v", mapFuncCalls)
	}
}
