package exp

import (
	"sync"
	"testing"
)

func TestLazySetAddRemoveEmpty(t *testing.T) {
	var s LazySet[int]
	if !s.Empty() {
		t.Fatal("new set is not empty")
	}
	if !s.Add(1) {
		t.Fatal("first Add(1) = false")
	}
	if s.Add(1) {
		t.Fatal("second Add(1) = true")
	}
	if s.Empty() {
		t.Fatal("set with one item is empty")
	}
	if !s.Remove(1) {
		t.Fatal("Remove(1) = false")
	}
	if !s.Empty() {
		t.Fatal("set after Remove is not empty")
	}
	if s.Remove(1) {
		t.Fatal("second Remove(1) = true")
	}
}

func TestLazySetConcurrent(t *testing.T) {
	var s LazySet[int]
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(v int) { defer wg.Done(); s.Add(v) }(i)
		go func(v int) { defer wg.Done(); s.Remove(v) }(i)
	}
	wg.Wait()
}

func TestLazyMapSetRangeDoesNotCreateGroups(t *testing.T) {
	var ms LazyMapSet[string, int]
	ms.RangeSet("missing", func(int) bool {
		t.Fatal("RangeSet visited a missing group")
		return false
	})
	if ms.Put("a", 1) != true {
		t.Fatal("Put(a,1) = false")
	}
	if ms.Put("a", 1) != false {
		t.Fatal("second Put(a,1) = true")
	}
	if !ms.Remove("a", 1) {
		t.Fatal("Remove(a,1) = false")
	}
	if ms.Remove("a", 1) {
		t.Fatal("second Remove(a,1) = true")
	}
}
