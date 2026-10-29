package infra

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/ez4bk/ezboot"
	"github.com/ez4bk/ezboot/internal/gin-boot/conf_type"
	"github.com/google/wire"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
)

type searchClients struct {
	mu sync.RWMutex

	clients map[string]*elasticsearch.Client
}

func (s *searchClients) open(cfg *config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients = make(map[string]*elasticsearch.Client)
	for name, c := range cfg.App.Middleware.Elasticsearch {
		if !c.Startup {
			continue
		}
		client, err := elasticsearch.NewClient(*elasticsearchConfig(&c))
		if err != nil {
			return resourceError("elasticsearch", name, err)
		}
		s.clients[name] = client
	}
	return nil
}

func (s *searchClients) get(name string) (*elasticsearch.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[name]
	if !ok {
		return nil, resourceError("elasticsearch", name, nil)
	}
	return client, nil
}

// close is a no-op for the Elasticsearch v7 client, which does not expose a
// Close method. It exists so searchClients satisfies the same lifecycle
// contract as the other resource groups.
func (s *searchClients) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients = nil
	return nil
}

func (s *searchClients) healthy(ctx context.Context) []error {
	s.mu.RLock()
	clients := make(map[string]*elasticsearch.Client, len(s.clients))
	for name, client := range s.clients {
		clients[name] = client
	}
	s.mu.RUnlock()

	var errs []error
	for name, client := range clients {
		res, err := esapi.InfoRequest{}.Do(ctx, client)
		if err != nil {
			errs = append(errs, fmt.Errorf("elasticsearch %q: %w", name, err))
			continue
		}
		if res.IsError() {
			errs = append(errs, fmt.Errorf("elasticsearch %q: %s", name, res.Status()))
		}
		_ = res.Body.Close()
	}
	return errs
}

// Elasticsearch returns a configured Elasticsearch client by name.
func (s *Elasticsearch) Elasticsearch(name string) (*elasticsearch.Client, error) {
	if s == nil {
		return nil, resourceError("elasticsearch", name, nil)
	}
	return s.searchClients.get(name)
}

// DefaultElasticsearch returns the default Elasticsearch client.
func (s *Elasticsearch) DefaultElasticsearch() (*elasticsearch.Client, error) {
	return s.Elasticsearch(defaultName)
}

func elasticsearchConfig(opt *conf_type.Elasticsearch) *elasticsearch.Config {
	return &elasticsearch.Config{
		Addresses: opt.Addresses,
		Username:  opt.Username,
		Password:  opt.Password,
	}
}

// Elasticsearch 是 Elasticsearch 插件。客户端在构造期创建。
type Elasticsearch struct {
	ezboot.PluginBase

	conf *config.Config
	searchClients
}

// ElasticsearchProviderSet 提供插件，以及 default 实例的客户端。
var ElasticsearchProviderSet = wire.NewSet(NewElasticsearch, ProvideDefaultElasticsearch)

// NewElasticsearch 创建插件并建立客户端。
func NewElasticsearch(cfg *config.Config) (*Elasticsearch, error) {
	if cfg == nil {
		return nil, errors.New("infra: elasticsearch requires a config")
	}
	if len(cfg.App.Middleware.Elasticsearch) == 0 {
		return nil, errors.New("infra: elasticsearch plugin is enabled but no instance is configured")
	}

	e := &Elasticsearch{conf: cfg}
	if err := e.open(cfg); err != nil {
		return nil, err
	}

	return e, nil
}

// Name 返回插件名。
func (e *Elasticsearch) Name() string { return "elasticsearch" }

// Start 阻塞到 ctx 结束。
func (e *Elasticsearch) Start(ctx context.Context) error {
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

// ProvideDefaultElasticsearch 提供 default 实例的客户端。
func ProvideDefaultElasticsearch(e *Elasticsearch) (*elasticsearch.Client, error) {
	return e.Elasticsearch(defaultName)
}
