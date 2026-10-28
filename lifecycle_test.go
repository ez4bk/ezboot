package ezboot

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakePlugin 是一个可编排的测试插件。未设置的行为使用默认实现：
// Start 阻塞到 ctx 结束，其余阶段直接返回 nil。
type fakePlugin struct {
	name string

	mu     sync.Mutex
	starts int
	readys int
	drains int
	closes int

	startFn func(context.Context) error
	readyFn func(context.Context) error
	drainFn func(context.Context) error
	closeFn func(context.Context) error
}

func (p *fakePlugin) Name() string { return p.name }

func (p *fakePlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	p.starts++
	fn := p.startFn
	p.mu.Unlock()

	if fn != nil {
		return fn(ctx)
	}

	<-ctx.Done()
	return nil
}

func (p *fakePlugin) Ready(ctx context.Context) error {
	p.mu.Lock()
	p.readys++
	fn := p.readyFn
	p.mu.Unlock()

	if fn != nil {
		return fn(ctx)
	}
	return nil
}

func (p *fakePlugin) Drain(ctx context.Context) error {
	p.mu.Lock()
	p.drains++
	fn := p.drainFn
	p.mu.Unlock()

	if fn != nil {
		return fn(ctx)
	}
	return nil
}

func (p *fakePlugin) Close(ctx context.Context) error {
	p.mu.Lock()
	p.closes++
	fn := p.closeFn
	p.mu.Unlock()

	if fn != nil {
		return fn(ctx)
	}
	return nil
}

func (p *fakePlugin) counts() (starts, readys, drains, closes int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.starts, p.readys, p.drains, p.closes
}

func runAsync(lc *lifecycle, ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	go func() { errc <- lc.run(ctx) }()
	return errc
}

func waitStarted(t *testing.T, st *State) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if st.Started() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("state did not become started")
}

func waitRun(t *testing.T, errc <-chan error) error {
	t.Helper()

	select {
	case err := <-errc:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("lifecycle did not finish in time")
		return nil
	}
}

func fastTimeouts() timeouts {
	return timeouts{ready: time.Second, drain: time.Second, close: time.Second}
}

func TestLifecycleStartsPluginsConcurrently(t *testing.T) {
	var (
		mu    sync.Mutex
		times []time.Time
	)

	block := func(ctx context.Context) error {
		mu.Lock()
		times = append(times, time.Now())
		mu.Unlock()
		<-ctx.Done()
		return nil
	}

	a := &fakePlugin{name: "a", startFn: block}
	b := &fakePlugin{name: "b", startFn: block}

	st := NewState()
	lc := newLifecycle(st, fastTimeouts(), []Plugin{a, b})

	ctx, cancel := context.WithCancel(context.Background())
	errc := runAsync(lc, ctx)
	waitStarted(t, st)

	mu.Lock()
	got := slices.Clone(times)
	mu.Unlock()

	if len(got) != 2 {
		t.Fatalf("expected 2 starts, got %d", len(got))
	}
	if d := got[1].Sub(got[0]); d > 100*time.Millisecond {
		t.Fatalf("starts were not concurrent, %v apart", d)
	}

	cancel()
	if err := waitRun(t, errc); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestLifecycleReadyIsOnlyCalledDuringStartup(t *testing.T) {
	p := &fakePlugin{name: "p"}
	st := NewState()
	lc := newLifecycle(st, fastTimeouts(), []Plugin{p})

	ctx, cancel := context.WithCancel(context.Background())
	errc := runAsync(lc, ctx)
	waitStarted(t, st)

	time.Sleep(250 * time.Millisecond)
	cancel()
	if err := waitRun(t, errc); err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, readys, _, _ := p.counts(); readys != 1 {
		t.Fatalf("expected Ready to be called once, got %d", readys)
	}
}

func TestLifecycleReadyTimeoutRollsBack(t *testing.T) {
	bad := &fakePlugin{
		name:    "bad",
		readyFn: func(context.Context) error { return errors.New("down") },
	}
	good := &fakePlugin{name: "good"}

	st := NewState()
	lc := newLifecycle(st, timeouts{
		ready: 150 * time.Millisecond,
		drain: time.Second,
		close: time.Second,
	}, []Plugin{good, bad})

	err := lc.run(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "bad") || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Started() {
		t.Fatal("state must not be started after a failed startup")
	}
	if _, _, _, closes := good.counts(); closes != 1 {
		t.Fatalf("expected the started plugin to be closed once, got %d", closes)
	}
}

func TestLifecycleStartFailureRollsBack(t *testing.T) {
	failing := &fakePlugin{
		name:    "failing",
		startFn: func(context.Context) error { return errors.New("boom") },
	}
	other := &fakePlugin{name: "other"}

	st := NewState()
	lc := newLifecycle(st, fastTimeouts(), []Plugin{other, failing})

	err := lc.run(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "failing") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, _, closes := other.counts(); closes != 1 {
		t.Fatalf("expected rollback close, got %d", closes)
	}
	if st.Started() {
		t.Fatal("state must not be started after a failed startup")
	}
}

func TestLifecycleRecoversFromPanics(t *testing.T) {
	panicking := &fakePlugin{
		name:    "panicking",
		startFn: func(context.Context) error { panic("kaboom") },
	}

	st := NewState()
	lc := newLifecycle(st, fastTimeouts(), []Plugin{panicking})

	err := lc.run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("expected a panic error, got %v", err)
	}
}

func TestLifecycleDrainsAndClosesInReverseOrder(t *testing.T) {
	var (
		mu         sync.Mutex
		drainOrder []string
		closeOrder []string
	)

	mk := func(name string) *fakePlugin {
		return &fakePlugin{
			name: name,
			drainFn: func(context.Context) error {
				mu.Lock()
				drainOrder = append(drainOrder, name)
				mu.Unlock()
				return nil
			},
			closeFn: func(context.Context) error {
				mu.Lock()
				closeOrder = append(closeOrder, name)
				mu.Unlock()
				return nil
			},
		}
	}

	st := NewState()
	lc := newLifecycle(st, fastTimeouts(), []Plugin{mk("a"), mk("b"), mk("c")})

	ctx, cancel := context.WithCancel(context.Background())
	errc := runAsync(lc, ctx)
	waitStarted(t, st)
	cancel()

	if err := waitRun(t, errc); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := []string{"c", "b", "a"}

	mu.Lock()
	gotDrain := slices.Clone(drainOrder)
	gotClose := slices.Clone(closeOrder)
	mu.Unlock()

	if !slices.Equal(gotDrain, want) {
		t.Fatalf("drain order = %v, want %v", gotDrain, want)
	}
	if !slices.Equal(gotClose, want) {
		t.Fatalf("close order = %v, want %v", gotClose, want)
	}
}

func TestLifecycleDrainUsesFreshContext(t *testing.T) {
	var (
		mu            sync.Mutex
		drainCtxErr   error
		drainDeadline bool
	)

	p := &fakePlugin{
		name: "p",
		drainFn: func(ctx context.Context) error {
			mu.Lock()
			drainCtxErr = ctx.Err()
			_, drainDeadline = ctx.Deadline()
			mu.Unlock()
			return nil
		},
	}

	st := NewState()
	lc := newLifecycle(st, fastTimeouts(), []Plugin{p})

	ctx, cancel := context.WithCancel(context.Background())
	errc := runAsync(lc, ctx)
	waitStarted(t, st)
	cancel()

	if err := waitRun(t, errc); err != nil {
		t.Fatalf("run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if drainCtxErr != nil {
		t.Fatalf("Drain received a cancelled context: %v", drainCtxErr)
	}
	if !drainDeadline {
		t.Fatal("Drain context has no deadline")
	}
}

func TestLifecycleShutsDownWhenPluginStops(t *testing.T) {
	p := &fakePlugin{
		name: "p",
		startFn: func(ctx context.Context) error {
			select {
			case <-time.After(50 * time.Millisecond):
				return errors.New("connection lost")
			case <-ctx.Done():
				return nil
			}
		},
	}

	st := NewState()
	lc := newLifecycle(st, fastTimeouts(), []Plugin{p})

	err := lc.run(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "stopped") || !strings.Contains(err.Error(), "connection lost") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _, drains, closes := p.counts()
	if drains != 1 || closes != 1 {
		t.Fatalf("expected drain and close, got drains=%d closes=%d", drains, closes)
	}
}

func TestLifecycleNoPlugins(t *testing.T) {
	st := NewState()
	lc := newLifecycle(st, fastTimeouts(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	errc := runAsync(lc, ctx)
	waitStarted(t, st)
	cancel()

	if err := waitRun(t, errc); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestStateTransitions(t *testing.T) {
	st := NewState()

	if err := st.Ready(); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("expected ErrNotStarted, got %v", err)
	}

	st.MarkStarted()
	if err := st.Ready(); err != nil {
		t.Fatalf("expected ready, got %v", err)
	}

	st.MarkDraining()
	if err := st.Ready(); !errors.Is(err, ErrDraining) {
		t.Fatalf("expected ErrDraining, got %v", err)
	}
	if st.Started() != true || st.Draining() != true {
		t.Fatal("unexpected state flags")
	}
}

func TestStateNilIsSafe(t *testing.T) {
	var st *State

	if st.Started() || st.Draining() {
		t.Fatal("nil state must report false")
	}
	if err := st.Ready(); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("expected ErrNotStarted, got %v", err)
	}

	st.MarkStarted()
	st.MarkDraining()
}

func TestPluginBaseStartBlocksUntilContextDone(t *testing.T) {
	var base PluginBase

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- base.Start(ctx) }()

	select {
	case <-done:
		t.Fatal("Start returned before the context was cancelled")
	case <-time.After(20 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return after the context was cancelled")
	}

	if err := base.Ready(context.Background()); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	if err := base.Drain(context.Background()); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if err := base.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
