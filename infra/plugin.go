package infra

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/ez4bk/ezboot"
	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/with"
)

func loggerOrDefault(logger *zap.SugaredLogger) *zap.SugaredLogger {
	if logger != nil {
		return logger
	}
	return zap.NewNop().Sugar()
}

// MySQL 是数据库插件。一个插件管理配置里所有 MySQL / SQLite / Pgsql
// 实例，与 `app.middleware.mysql` 等配置段一一对应。
//
// Start 打开连接池并把它注册为 with 包的默认数据库；Ready 对 default
// 实例做一次 Ping。
type MySQL struct {
	ezboot.PluginBase

	conf   *config.Config
	logger *zap.SugaredLogger
	dbConns
}

// NewMySQL 创建一个数据库插件。配置里没有任何数据库实例时返回错误，
// 避免“声明了插件但什么都没配”的静默失败。
func NewMySQL(cfg *config.Config, logger *zap.SugaredLogger) (*MySQL, error) {
	if cfg == nil {
		return nil, errors.New("infra: mysql requires a config")
	}
	if len(cfg.App.Middleware.Mysql)+len(cfg.App.Middleware.Sqlite)+len(cfg.App.Middleware.Pgsql) == 0 {
		return nil, errors.New("infra: mysql plugin is enabled but no database is configured")
	}
	return &MySQL{conf: cfg, logger: loggerOrDefault(logger)}, nil
}

// Name 返回插件名。
func (m *MySQL) Name() string { return "mysql" }

// Start 打开连接池，注册默认数据库，然后阻塞到 ctx 结束。
func (m *MySQL) Start(ctx context.Context) error {
	if err := m.open(m.conf, m.logger); err != nil {
		return err
	}
	if engine, err := m.engine(defaultName); err == nil {
		with.SetDefault(engine)
	}

	<-ctx.Done()
	return nil
}

// Ready 对已打开的数据库做一次 Ping。
func (m *MySQL) Ready(ctx context.Context) error {
	return errors.Join(m.healthy(ctx)...)
}

// Close 释放所有连接池。
func (m *MySQL) Close(context.Context) error {
	if m == nil {
		return nil
	}
	return m.close()
}

// Redis 是 Redis 插件。Start 创建客户端并注册 with 包的默认 Redis。
type Redis struct {
	ezboot.PluginBase

	conf *config.Config
	redisClients
}

// NewRedis 创建一个 Redis 插件。
func NewRedis(cfg *config.Config) (*Redis, error) {
	if cfg == nil {
		return nil, errors.New("infra: redis requires a config")
	}
	if len(cfg.App.Middleware.Redis) == 0 {
		return nil, errors.New("infra: redis plugin is enabled but no instance is configured")
	}
	return &Redis{conf: cfg}, nil
}

// Name 返回插件名。
func (r *Redis) Name() string { return "redis" }

// Start 建立客户端，注册默认 Redis，然后阻塞到 ctx 结束。
func (r *Redis) Start(ctx context.Context) error {
	if err := r.open(r.conf); err != nil {
		return err
	}
	if client, err := r.get(defaultName); err == nil {
		with.SetDefaultRedis(client)
	}

	<-ctx.Done()
	return nil
}

// Ready 对已配置的 Redis 实例做一次 Ping。
func (r *Redis) Ready(ctx context.Context) error {
	return errors.Join(r.healthy(ctx)...)
}

// Close 关闭所有客户端。
func (r *Redis) Close(context.Context) error {
	if r == nil {
		return nil
	}
	return r.close()
}

// Kafka 是 Kafka 插件。它只保存 broker 列表，reader / writer 由调用方
// 通过 KafkaReader / KafkaWriter 按需创建。
type Kafka struct {
	ezboot.PluginBase

	conf *config.Config
	kafkaClients
}

// NewKafka 创建一个 Kafka 插件。
func NewKafka(cfg *config.Config) (*Kafka, error) {
	if cfg == nil {
		return nil, errors.New("infra: kafka requires a config")
	}
	if len(cfg.App.Middleware.Kafka.KafkaClusterURL) == 0 {
		return nil, errors.New("infra: kafka plugin is enabled but no broker is configured")
	}
	return &Kafka{conf: cfg}, nil
}

// Name 返回插件名。
func (k *Kafka) Name() string { return "kafka" }

// Start 记录 broker 列表，然后阻塞到 ctx 结束。
func (k *Kafka) Start(ctx context.Context) error {
	if err := k.open(k.conf); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

// Ready 探测已配置的 broker。
func (k *Kafka) Ready(ctx context.Context) error {
	return errors.Join(k.healthy(ctx)...)
}

// Close 释放 broker 句柄。
func (k *Kafka) Close(context.Context) error {
	if k == nil {
		return nil
	}
	return k.close()
}

// RabbitMQ 是 RabbitMQ 插件。Start 建立连接并启动重连循环。
type RabbitMQ struct {
	ezboot.PluginBase

	conf   *config.Config
	logger *zap.SugaredLogger
	rabbitMQ
}

// NewRabbitMQ 创建一个 RabbitMQ 插件。
func NewRabbitMQ(cfg *config.Config, logger *zap.SugaredLogger) (*RabbitMQ, error) {
	if cfg == nil {
		return nil, errors.New("infra: rabbitmq requires a config")
	}
	if cfg.App.Middleware.RabbitMQ.Url == "" {
		return nil, errors.New("infra: rabbitmq plugin is enabled but url is empty")
	}
	return &RabbitMQ{conf: cfg, logger: loggerOrDefault(logger)}, nil
}

// Name 返回插件名。
func (r *RabbitMQ) Name() string { return "rabbitmq" }

// Start 建立连接并启动重连循环，然后阻塞到 ctx 结束。
func (r *RabbitMQ) Start(ctx context.Context) error {
	if err := r.open(r.conf, r.logger); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

// Ready 检查已配置的 publisher / consumer 是否就绪。
func (r *RabbitMQ) Ready(ctx context.Context) error {
	return errors.Join(r.healthy(ctx)...)
}

// Close 关闭连接。
func (r *RabbitMQ) Close(context.Context) error {
	if r == nil {
		return nil
	}
	return r.close()
}

// Elasticsearch 是 Elasticsearch 插件。
type Elasticsearch struct {
	ezboot.PluginBase

	conf *config.Config
	searchClients
}

// NewElasticsearch 创建一个 Elasticsearch 插件。
func NewElasticsearch(cfg *config.Config) (*Elasticsearch, error) {
	if cfg == nil {
		return nil, errors.New("infra: elasticsearch requires a config")
	}
	if len(cfg.App.Middleware.Elasticsearch) == 0 {
		return nil, errors.New("infra: elasticsearch plugin is enabled but no instance is configured")
	}
	return &Elasticsearch{conf: cfg}, nil
}

// Name 返回插件名。
func (e *Elasticsearch) Name() string { return "elasticsearch" }

// Start 创建客户端，然后阻塞到 ctx 结束。
func (e *Elasticsearch) Start(ctx context.Context) error {
	if err := e.open(e.conf); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

// Ready 对已配置的集群做一次探活。
func (e *Elasticsearch) Ready(ctx context.Context) error {
	return errors.Join(e.healthy(ctx)...)
}

// Close 释放客户端。
func (e *Elasticsearch) Close(context.Context) error {
	if e == nil {
		return nil
	}
	return e.close()
}

// Etcd 是 etcd 插件。远程配置中心也挂在这个插件上。
type Etcd struct {
	ezboot.PluginBase

	conf *config.Config
	etcdClient
}

// NewEtcd 创建一个 etcd 插件。
func NewEtcd(cfg *config.Config) (*Etcd, error) {
	if cfg == nil {
		return nil, errors.New("infra: etcd requires a config")
	}
	if len(cfg.App.Middleware.Etcd.Endpoints) == 0 {
		return nil, errors.New("infra: etcd plugin is enabled but no endpoint is configured")
	}
	return &Etcd{conf: cfg}, nil
}

// Name 返回插件名。
func (e *Etcd) Name() string { return "etcd" }

// Start 建立客户端，然后阻塞到 ctx 结束。
func (e *Etcd) Start(ctx context.Context) error {
	if err := e.open(e.conf); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

// Ready 探测已配置的 endpoint。
func (e *Etcd) Ready(ctx context.Context) error {
	return errors.Join(e.healthy(ctx)...)
}

// Close 关闭客户端。
func (e *Etcd) Close(context.Context) error {
	if e == nil {
		return nil
	}
	return e.close()
}
