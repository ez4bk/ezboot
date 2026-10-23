package exp

import (
	"iter"
	"sync"
)

type Map[K comparable, V any] struct {
	underlying sync.Map
}

func NewMap[K comparable, V any]() Map[K, V] {
	return Map[K, V]{}
}

func (m *Map[K, V]) Get(k K) (v V, _ bool) {
	if value, ok := m.underlying.Load(k); ok {
		return value.(V), true
	}

	return
}

func (m *Map[K, V]) Set(k K, v V) {
	m.underlying.Store(k, v)
}

func (m *Map[K, V]) LoadOrStore(k K, v V) (actual V, loaded bool) {
	value, loaded := m.underlying.LoadOrStore(k, v)
	return value.(V), loaded
}

func (m *Map[K, V]) Delete(k K) {
	m.underlying.Delete(k)
}

func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.underlying.Range(func(k, v any) bool {
			return yield(k.(K), v.(V))
		})
	}
}

func (m *Map[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		m.underlying.Range(func(k, _ any) bool {
			return yield(k.(K))
		})
	}
}
