package ezboot

import (
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
