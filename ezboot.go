package ezboot

import (
	"context"
	"errors"

	"github.com/go-redis/redis"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	boot "github.com/ez4bk/ezboot/internal/gin-boot"
	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/util"
	"github.com/ez4bk/ezboot/internal/logging"
)

// Config 配置别名
type Config = config.Config

// ProvideConfig 加载框架全局配置。
//
// 它同时完成需要早于插件构造的初始化（tracing 拦截器、gRPC 客户端池），
// 因为 gRPC driver 在构造时就会读取拦截器列表。
func ProvideConfig() *Config {
	util.ChkErr(boot.LoadFrameworkConfig())
	util.ChkErr(boot.InitTelemetry())

	return &boot.Config
}

// Logger 日志实现的别名
type Logger = zap.SugaredLogger

// ProvideLogger 加载日志记录器
func ProvideLogger() *Logger {
	// log.With(app, xxx)
	return logging.Default()
}

// DiscardLogger 是一个默认的空输出的日志记录器
var DiscardLogger = zap.NewNop().Sugar()

// ErrResourceUnavailable 表示进程级基础设施不再由框架持有：资源现在由
// 各自的插件负责，需要通过 ezboot/infra 引入。
var ErrResourceUnavailable = errors.New("ezboot: resource unavailable")

// ProvideRedis 已废弃。
//
// Deprecated: 加入 infra.Redis 插件，并通过该插件的 Redis(name) 访问客户端。
func ProvideRedis() (*redis.Client, error) {
	return nil, ErrResourceUnavailable
}

// Healthy 已废弃。
//
// Deprecated: 就绪状态现在由 ezboot.State 提供；依赖探活由各插件的 Ready
// 在启动阶段完成。
func Healthy(context.Context) error {
	return ErrResourceUnavailable
}

// LoadUserConfig 加载用户配置
func LoadUserConfig(cfg any) error {
	// ensure the config files and viper are initialized; this must not open
	// any infrastructure resource
	if err := boot.LoadFrameworkConfig(); err != nil {
		return err
	}

	return viper.Unmarshal(cfg, func(config *mapstructure.DecoderConfig) {
		config.TagName = "yaml"
		config.Metadata = &mapstructure.Metadata{}
	})
}
