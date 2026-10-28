package ezboot

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ez4bk/ezboot/grpc"
	"github.com/ez4bk/ezboot/http"
	boot "github.com/ez4bk/ezboot/internal/gin-boot"
)

// Application 是一个gin-boot应用程序
type Application struct {
	name string

	plugins  []Plugin
	state    *State
	timeouts timeouts

	// 兼容旧的装配方式：WithHttpServer / WithGrpcServer
	hs *http.Server
	gs *grpc.Server

	hooks struct {
		pres []func(ctx context.Context) error
	}

	root *cobra.Command
}

// Start 启动gin-boot应用
func (app *Application) Start(ctx context.Context) error {
	return app.root.ExecuteContext(ctx)
}

// serve 启用应用并开始处理请求
func (app *Application) serve(ctx context.Context) error {
	for _, hook := range app.hooks.pres {
		if err := hook(ctx); err != nil {
			return err
		}
	}

	if err := boot.Migrate(); err != nil {
		return err
	}

	plugins := make([]Plugin, 0, len(app.plugins)+2)
	plugins = append(plugins, app.plugins...)
	if app.gs != nil {
		plugins = append(plugins, app.gs)
	}
	if app.hs != nil {
		plugins = append(plugins, app.hs)
	}

	return newLifecycle(app.state, app.timeouts, plugins).run(ctx)
}

// ApplicationOption 创建应用的额外选项
type ApplicationOption func(app *Application) error

// WithAppName 设置应用名字
func WithAppName(name string) ApplicationOption {
	return func(app *Application) error {
		app.name = name
		return nil
	}
}

// WithPlugins 注册参与生命周期的插件。列表顺序决定 Close 的逆序：
// 越靠前的插件越晚关闭（例如 OTEL 应放在最前）。
func WithPlugins(plugins ...Plugin) ApplicationOption {
	return func(app *Application) error {
		app.plugins = append(app.plugins, plugins...)
		return nil
	}
}

// WithState 设置就绪状态对象。它与 HTTP 插件共享，由 lifecycle 写入。
func WithState(state *State) ApplicationOption {
	return func(app *Application) error {
		if state == nil {
			return ErrNilState
		}
		app.state = state
		return nil
	}
}

// WithPluginTimeouts 覆盖生命周期各阶段的默认预算。
func WithPluginTimeouts(ready, drain, close time.Duration) ApplicationOption {
	return func(app *Application) error {
		app.timeouts = timeouts{ready: ready, drain: drain, close: close}
		return nil
	}
}

// WithHttpServer 设置启动Http服务。
//
// Deprecated: 使用 WithPlugins 显式注册插件。
func WithHttpServer(hs *http.Server) ApplicationOption {
	return func(app *Application) error {
		app.hs = hs
		return nil
	}
}

// WithGrpcServer 设置启动Grpc服务。
//
// Deprecated: 使用 WithPlugins 显式注册插件。
func WithGrpcServer(gs *grpc.Server) ApplicationOption {
	return func(app *Application) error {
		app.gs = gs
		return nil
	}
}

// WithStartPreHook 在服务启动之前的任务
func WithStartPreHook(hook func(context.Context) error) ApplicationOption {
	return func(app *Application) error {
		app.hooks.pres = append(app.hooks.pres, hook)
		return nil
	}
}

// WithCmdline 增加一个额外命令行配置项
func WithCmdline(cmdline ...*cobra.Command) ApplicationOption {
	return func(app *Application) error {
		app.root.AddCommand(cmdline...)
		return nil
	}
}

// NewApp 创建一个应用
func NewApp(options ...ApplicationOption) (*Application, error) {
	app := &Application{}
	app.root = &cobra.Command{
		Use:   os.Args[0],
		Short: "Start and process the incoming request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.serve(cmd.Context())
		},
	}

	for _, option := range options {
		if err := option(app); err != nil {
			return nil, err
		}
	}

	return app, nil
}
