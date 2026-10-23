package ezboot

import (
	"github.com/go-redis/redis"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/ez4bk/ezboot/internal/gin-boot/infra"

	boot "github.com/ez4bk/ezboot/internal/gin-boot"
	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/logging"
)

// Config 配置别名
type Config = config.Config

// ProvideConfig 加载框架全局配置
func ProvideConfig() *Config {
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

// ProvideRedis 加载 Redis 依赖。需要在 Wire 中使用 Redis 时，把
// ezboot.ProvideRedis 加进调用方的 wire.Build。
func ProvideRedis() (*redis.Client, error) {
	_ = boot.Default()
	return infra.Default().Redis("default")
}

type RabbitMQClient = infra.RabbitMQClient
type RabbitMQPublisher = infra.RabbitMQPublisher
type RabbitMQConsumer = infra.RabbitMQConsumer

// ProvideRabbitMQ 加载 RabbitMQ 依赖。需要在 Wire 中使用 RabbitMQ 时，
// 把 ezboot.ProvideRabbitMQ 加进调用方的 wire.Build。
func ProvideRabbitMQ() (*RabbitMQClient, error) {
	_ = boot.Default()
	return infra.Default().RabbitMQ()
}

// LoadUserConfig 加载用户配置
func LoadUserConfig(cfg any) error {
	// ensure env initialized
	_ = boot.Default()

	return viper.Unmarshal(cfg, func(config *mapstructure.DecoderConfig) {
		config.TagName = "yaml"
		config.Metadata = &mapstructure.Metadata{}
	})
}
