package ezboot

import "context"

// Plugin 是应用生命周期中一个可插拔的组件。
//
// 阶段调用顺序：Start → 轮询 Ready → Drain → Close。
//
// 契约：
//   - Start 必须阻塞到插件停止（覆盖它时记得自己写 <-ctx.Done()）。
//     Start 返回即代表插件已经停止。
//   - Ready 回答“现在能不能接流量”，只在启动期被调用。它必须便宜、
//     可重复调用、并发安全，不得自己重试或长时间阻塞。
//   - Drain 与 Close 收到的是 lifecycle 新建的、带超时的 ctx，不是
//     Start 时的那个。Drain 负责停止接收新工作并等待在途工作结束，
//     Close 负责释放资源。两者不可合并。
//   - Name 只用于日志与错误信息，不作为查找键。
type Plugin interface {
	Name() string
	Start(ctx context.Context) error
	Ready(ctx context.Context) error
	Drain(ctx context.Context) error
	Close(ctx context.Context) error
}

// PluginBase 提供四个阶段的默认实现。嵌入它之后，插件只需要实现
// Name，以及自己真正关心的那几个阶段。
type PluginBase struct{}

// Start 阻塞到 ctx 结束。这是有意为之：Start 返回就代表插件已经停止，
// lifecycle 依靠这个信号判断插件是否还活着。
func (PluginBase) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Ready 默认认为插件随时可用。有连接语义的插件必须覆盖它。
func (PluginBase) Ready(context.Context) error { return nil }

// Drain 默认什么都不做。
func (PluginBase) Drain(context.Context) error { return nil }

// Close 默认什么都不做。
func (PluginBase) Close(context.Context) error { return nil }
