package ezboot

import (
	"github.com/go-redis/redis"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/ez4bk/ezboot/internal/gin-boot/middleware"

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

// ProvideRedis 加载Redis依赖
func ProvideRedis() *redis.Client {
	return boot.MW.DefaultRedis()
}

type RabbitMQClient = middleware.RabbitMQClient
type RabbitMQPublisher = middleware.RabbitMQPublisher
type RabbitMQConsumer = middleware.RabbitMQConsumer

// ProvideRabbitMQ 加载RabbitMQ依赖
func ProvideRabbitMQ() *RabbitMQClient {
	return (*RabbitMQClient)(boot.MW.RabbitMQ())
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
