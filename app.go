package ezboot

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/ez4bk/ezboot/grpc"
	"github.com/ez4bk/ezboot/http"
)

// Application 是一个gin-boot应用程序
type Application struct {
	name string

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

// Start 启用应用并开始处理请求
func (app *Application) serve(ctx context.Context) error {
	eg, gCtx := errgroup.WithContext(ctx)

	for _, hook := range app.hooks.pres {
		if err := hook(gCtx); err != nil {
			return err
		}
	}

	if app.hs != nil {
		eg.Go(func() error { return app.hs.Start(gCtx) })
	}
	if app.gs != nil {
		eg.Go(func() error { return app.gs.Start(gCtx) })
	}

	return eg.Wait()
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

// WithHttpServer 设置启动Http服务
func WithHttpServer(hs *http.Server) ApplicationOption {
	return func(app *Application) error {
		app.hs = hs
		return nil
	}
}

// WithGrpcServer 设置启动Grpc服务
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
