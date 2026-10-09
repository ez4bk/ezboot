package exp

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// LazySet 是一个延迟初始化的集合
type LazySet[V any] struct {
	size     uint64
	lazyInit sync.Once
	lazyM    *sync.Map
}

// Add 把一个值加入到集合中, 返回值表示本次是否加入到集合中
func (s *LazySet[V]) Add(v V) bool {
	s.init()
	// The loaded result is true if the value was loaded, false if stored.
	if _, ok := s.lazyM.LoadOrStore(v, struct{}{}); !ok {
		atomic.AddUint64(&s.size, 1)
		return true
	}
	return false
}

// Remove 从集合中删除一个值, 返回值表示是否成功删除
func (s *LazySet[V]) Remove(v any) bool {
	s.init()
	// The loaded result reports whether the key was present.
	if _, ok := s.lazyM.LoadAndDelete(v); ok {
		atomic.CompareAndSwapUint64(&s.size, s.size, s.size-1)
		return true
	}
	return false
}

// Range 遍历集合中的所有元素, walk 函数的返回值表示是否继续遍历
func (s *LazySet[V]) Range(walk func(v V) bool) {
	s.init()
	// If f returns false, range stops the iteration.
	s.lazyM.Range(func(key, _ any) bool {
		return walk(key.(V))
	})
}

// Empty 返回当前集合是否为空
func (s *LazySet[V]) Empty() bool {
	return atomic.LoadUint64(&s.size) > 0
}

func (s *LazySet[V]) init() {
	s.lazyInit.Do(func() {
		s.lazyM = new(sync.Map)
	})
}

// LazyMapSet 是一个延迟的字典集合列表
type LazyMapSet[K any, V any] struct {
	c uint32
	m sync.Map
}

// Put 把一个值加入到键对应的集合中
func (ms *LazyMapSet[K, V]) Put(key K, val V) bool {
	ms.cleanup()
	s, _ := ms.m.LoadOrStore(key, &LazySet[V]{})
	return s.(*LazySet[V]).Add(val)
}

// Remove 从键对应的集合中删除一个值
func (ms *LazyMapSet[K, V]) Remove(key K, val V) bool {
	ms.cleanup()
	s, _ := ms.m.LoadOrStore(key, &LazySet[V]{})
	return s.(*LazySet[V]).Remove(val)
}

// RangeSet 遍历键对应集合中的所有元素, walk 函数的返回值表示是否继续遍历
func (ms *LazyMapSet[K, V]) RangeSet(key K, walk func(v V) bool) {
	ms.cleanup()
	if s, loaded := ms.m.LoadOrStore(key, &LazySet[V]{}); loaded {
		s.(*LazySet[V]).Range(walk)
	}
}

// cleanup 自动清理一些空闲的集合对象
func (ms *LazyMapSet[K, V]) cleanup() {
	if rand.Intn(100) == 0 {
		if atomic.CompareAndSwapUint32(&ms.c, 0, 1) {
			go func() {
				defer atomic.CompareAndSwapUint32(&ms.c, 1, 0)

				limit := 16
				ms.m.Range(func(key, value any) bool {
					if value.(*LazySet[V]).Empty() {
						ms.m.Delete(key)
					}
					limit--
					return limit < 0
				})
			}()
		}
	}
}
