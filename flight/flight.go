package flight

import "golang.org/x/sync/singleflight"

// Group represents a class of work and forms a namespace in
// which units of work can be executed with duplicate suppression.
type Group[T any] struct {
	underlying singleflight.Group
}

// Do execute and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
// The return value shared indicates whether v was given to multiple callers.
func (g *Group[T]) Do(key string, fn func() (T, error)) (v T, err error, shared bool) {
	var raw any
	raw, err, shared = g.underlying.Do(key, func() (interface{}, error) {
		return fn()
	})

	if raw != nil {
		v = raw.(T)
	}

	return
}

// Result holds the results of Do, so they can be passed
// on a channel.
type Result[T any] struct {
	Val    T
	Err    error
	Shared bool
}

// DoChan is like Do but returns a channel that will receive the
// results when they are ready.
//
// The returned channel will not be closed.
func (g *Group[T]) DoChan(key string, fn func() (T, error)) <-chan Result[T] {
	raws := g.underlying.DoChan(key, func() (interface{}, error) {
		return fn()
	})

	ch := make(chan Result[T])
	go func() {
		defer close(ch)

		for raw := range raws {
			ch <- Result[T]{
				Val:    raw.Val.(T),
				Err:    raw.Err,
				Shared: raw.Shared,
			}
		}
	}()

	return ch
}

// Forget tells the singleflight to forget about a key.  Future calls
// to Do for this key will call the function rather than waiting for
// an earlier call to complete.
func (g *Group[T]) Forget(key string) {
	g.underlying.Forget(key)
}
