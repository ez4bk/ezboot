package infra

import (
	"context"
	"fmt"
	"sync"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/ez4bk/ezboot/internal/gin-boot/conf_type"
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
func (s *Set) Elasticsearch(name string) (*elasticsearch.Client, error) {
	if s == nil {
		return nil, resourceError("elasticsearch", name, nil)
	}
	return s.search.get(name)
}

// DefaultElasticsearch returns the default Elasticsearch client.
func (s *Set) DefaultElasticsearch() (*elasticsearch.Client, error) {
	return s.Elasticsearch(defaultName)
}

func elasticsearchConfig(opt *conf_type.Elasticsearch) *elasticsearch.Config {
	return &elasticsearch.Config{
		Addresses: opt.Addresses,
		Username:  opt.Username,
		Password:  opt.Password,
	}
}
