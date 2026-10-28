package ezboot

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	defaultReadyTimeout = 10 * time.Second
	defaultDrainTimeout = 10 * time.Second
	defaultCloseTimeout = 5 * time.Second

	// readyAttemptTimeout 是单次 Ready 调用的上限，防止插件把自己的
	// 探活阻塞带进启动流程；总预算由 timeouts.ready 控制。
	readyAttemptTimeout = 2 * time.Second
	readyPollInterval   = 100 * time.Millisecond
)

// timeouts 是生命周期各阶段的预算，零值表示使用默认值。
type timeouts struct {
	ready time.Duration
	drain time.Duration
	close time.Duration
}

func (t timeouts) withDefaults() timeouts {
	if t.ready <= 0 {
		t.ready = defaultReadyTimeout
	}
	if t.drain <= 0 {
		t.drain = defaultDrainTimeout
	}
	if t.close <= 0 {
		t.close = defaultCloseTimeout
	}
	return t
}

// startResult 是插件 Start 返回的结果。收到它就意味着插件已经停止。
type startResult struct {
	name string
	err  error
}

// lifecycle 是按阶段编排插件启停的状态机，不导出：调用方只通过
// Application 使用它。
//
// 阶段：并发 Start → 轮询 Ready → MarkStarted → 等待退出信号 →
// MarkDraining → 逆序 Drain → 取消 Start 的 ctx → 逆序 Close。
type lifecycle struct {
	plugins  []Plugin
	state    *State
	timeouts timeouts
}

func newLifecycle(state *State, ts timeouts, plugins []Plugin) *lifecycle {
	if state == nil {
		state = NewState()
	}
	return &lifecycle{
		plugins:  plugins,
		state:    state,
		timeouts: ts.withDefaults(),
	}
}

// run 驱动整个生命周期，直到 ctx 结束或某个插件提前停止。
//
// 启动失败时会回滚：取消仍在运行的插件并关闭所有已启动的插件。
func (l *lifecycle) run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	started := make(chan startResult, len(l.plugins))

	for _, p := range l.plugins {
		wg.Add(1)
		go func(p Plugin) {
			defer wg.Done()
			started <- startResult{name: p.Name(), err: callStart(runCtx, p)}
		}(p)
	}

	if err := l.waitReady(ctx, started); err != nil {
		cancel()
		return errors.Join(err, l.stopStarts(&wg), l.closeAll())
	}

	l.state.MarkStarted()

	var runErr error
	select {
	case <-ctx.Done():
	case r := <-started:
		// ctx 已经结束时插件返回属于正常停止，不算故障。
		if ctx.Err() == nil {
			runErr = startStopError(r.name, r.err)
		}
	}

	l.state.MarkDraining()

	// 先 Drain，让在途工作结束；再取消 ctx，解锁那些只等 ctx 的插件。
	drainErr := l.drainAll()
	cancel()
	stopErr := l.stopStarts(&wg)
	closeErr := l.closeAll()

	return errors.Join(runErr, drainErr, stopErr, closeErr)
}

// waitReady 并发轮询每个插件的 Ready，直到全部就绪、超时、ctx 结束，
// 或某个插件在就绪前就停止了。
func (l *lifecycle) waitReady(ctx context.Context, started <-chan startResult) error {
	readyCtx, cancel := context.WithTimeout(ctx, l.timeouts.ready)
	defer cancel()

	readyErrs := make(chan error, len(l.plugins))
	for _, p := range l.plugins {
		go func(p Plugin) {
			readyErrs <- pollReady(readyCtx, p)
		}(p)
	}

	for range l.plugins {
		select {
		case <-ctx.Done():
			return fmt.Errorf("ezboot: startup cancelled: %w", ctx.Err())
		case r := <-started:
			return startStopError(r.name, r.err)
		case err := <-readyErrs:
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// pollReady 反复调用 Ready，直到成功或 ctx 超时。
func pollReady(ctx context.Context, p Plugin) error {
	var lastErr error

	for {
		attemptCtx, cancel := context.WithTimeout(ctx, readyAttemptTimeout)
		err := callReady(attemptCtx, p)
		cancel()

		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return fmt.Errorf("ezboot: plugin %q is not ready: %w", p.Name(), lastErr)
		case <-time.After(readyPollInterval):
		}
	}
}

// drainAll 按启动的逆序排空插件。每个插件拿到独立的、带超时的 ctx。
func (l *lifecycle) drainAll() error {
	var errs []error

	for i := len(l.plugins) - 1; i >= 0; i-- {
		p := l.plugins[i]

		ctx, cancel := context.WithTimeout(context.Background(), l.timeouts.drain)
		err := callDrain(ctx, p)
		cancel()

		if err != nil {
			errs = append(errs, fmt.Errorf("ezboot: plugin %q drain failed: %w", p.Name(), err))
		}
	}

	return errors.Join(errs...)
}

// closeAll 按启动的逆序释放插件。每个插件拿到独立的、带超时的 ctx。
func (l *lifecycle) closeAll() error {
	var errs []error

	for i := len(l.plugins) - 1; i >= 0; i-- {
		p := l.plugins[i]

		ctx, cancel := context.WithTimeout(context.Background(), l.timeouts.close)
		err := callClose(ctx, p)
		cancel()

		if err != nil {
			errs = append(errs, fmt.Errorf("ezboot: plugin %q close failed: %w", p.Name(), err))
		}
	}

	return errors.Join(errs...)
}

// stopStarts 等待所有 Start 返回，最多等一个 drain 预算。
func (l *lifecycle) stopStarts(wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(l.timeouts.drain):
		return errors.New("ezboot: plugins did not stop within the drain timeout")
	}
}

func startStopError(name string, err error) error {
	if err == nil {
		return fmt.Errorf("ezboot: plugin %q stopped unexpectedly", name)
	}
	return fmt.Errorf("ezboot: plugin %q stopped unexpectedly: %w", name, err)
}

func callStart(ctx context.Context, p Plugin) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("ezboot: plugin %q panicked in Start: %v", p.Name(), v)
		}
	}()
	return p.Start(ctx)
}

func callReady(ctx context.Context, p Plugin) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("ezboot: plugin %q panicked in Ready: %v", p.Name(), v)
		}
	}()
	return p.Ready(ctx)
}

func callDrain(ctx context.Context, p Plugin) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("ezboot: plugin %q panicked in Drain: %v", p.Name(), v)
		}
	}()
	return p.Drain(ctx)
}

func callClose(ctx context.Context, p Plugin) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("ezboot: plugin %q panicked in Close: %v", p.Name(), v)
		}
	}()
	return p.Close(ctx)
}
