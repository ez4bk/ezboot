package ezboot

import (
	"errors"
	"sync/atomic"
)

var (
	// ErrNotStarted 表示应用还没有完成启动，不应接收流量。
	ErrNotStarted = errors.New("ezboot: not started")
	// ErrDraining 表示应用正在排空，不应接收新流量。
	ErrDraining = errors.New("ezboot: draining")
)

// State 是就绪状态的唯一数据源：由 lifecycle 写入，由就绪端点读取。
//
// 运行期的 /readyz 只读它，不去探测任何插件。依赖故障交给超时、重试
// 与熔断处理；readiness 只回答“这个实例该不该接流量”。
type State struct {
	started  atomic.Bool
	draining atomic.Bool
}

// NewState 创建一个就绪状态对象。
func NewState() *State {
	return &State{}
}

// MarkStarted 标记启动完成。由 lifecycle 在全部插件 Ready 之后调用。
func (s *State) MarkStarted() {
	if s != nil {
		s.started.Store(true)
	}
}

// MarkDraining 标记开始排空。调用后 /readyz 立即返回 503。
func (s *State) MarkDraining() {
	if s != nil {
		s.draining.Store(true)
	}
}

// Started 报告启动是否已完成。
func (s *State) Started() bool {
	return s != nil && s.started.Load()
}

// Draining 报告是否正在排空。
func (s *State) Draining() bool {
	return s != nil && s.draining.Load()
}

// Ready 返回应用当前是否可以接收流量。它可以直接作为 /readyz 的实现。
func (s *State) Ready() error {
	if s == nil || !s.started.Load() {
		return ErrNotStarted
	}
	if s.draining.Load() {
		return ErrDraining
	}
	return nil
}
