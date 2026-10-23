package exp

import "sync"

// LazySet is a concurrency-safe set that allocates its backing map on first
// use.
type LazySet[V comparable] struct {
	mu sync.RWMutex
	m  map[V]struct{}
}

// Add inserts v and reports whether it was newly added.
func (s *LazySet[V]) Add(v V) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.m == nil {
		s.m = make(map[V]struct{})
	}
	if _, ok := s.m[v]; ok {
		return false
	}
	s.m[v] = struct{}{}
	return true
}

// Remove deletes v and reports whether it was present.
func (s *LazySet[V]) Remove(v any) bool {
	key, ok := v.(V)
	if !ok {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; !ok {
		return false
	}
	delete(s.m, key)
	return true
}

// Range calls walk for each item until walk returns false.
func (s *LazySet[V]) Range(walk func(v V) bool) {
	s.mu.RLock()
	items := make([]V, 0, len(s.m))
	for v := range s.m {
		items = append(items, v)
	}
	s.mu.RUnlock()

	for _, v := range items {
		if !walk(v) {
			return
		}
	}
}

// Empty reports whether the set has no items.
func (s *LazySet[V]) Empty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m) == 0
}

// LazyMapSet groups sets by key and allocates each group on first use.
type LazyMapSet[K comparable, V comparable] struct {
	mu sync.Mutex
	m  map[K]*LazySet[V]
}

// Put adds val to the set for key and reports whether it was newly added.
func (ms *LazyMapSet[K, V]) Put(key K, val V) bool {
	ms.mu.Lock()
	if ms.m == nil {
		ms.m = make(map[K]*LazySet[V])
	}
	s := ms.m[key]
	if s == nil {
		s = &LazySet[V]{}
		ms.m[key] = s
	}
	added := s.Add(val)
	ms.mu.Unlock()

	return added
}

// Remove deletes val from the set for key and reports whether it was present.
func (ms *LazyMapSet[K, V]) Remove(key K, val V) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	s := ms.m[key]
	if s == nil {
		return false
	}

	removed := s.Remove(val)
	if removed && s.Empty() {
		delete(ms.m, key)
	}
	return removed
}

// RangeSet calls walk for each item in key's set until walk returns false.
// Missing keys are a no-op rather than creating an empty group.
func (ms *LazyMapSet[K, V]) RangeSet(key K, walk func(v V) bool) {
	ms.mu.Lock()
	s := ms.m[key]
	ms.mu.Unlock()
	if s == nil {
		return
	}
	s.Range(walk)
}
