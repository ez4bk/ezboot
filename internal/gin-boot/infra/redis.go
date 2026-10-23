package infra

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis"

	conftype "github.com/ez4bk/ezboot/internal/gin-boot/conf_type"
	"github.com/ez4bk/ezboot/internal/gin-boot/config"
)

type redisClients struct {
	mu sync.RWMutex

	clients map[string]*redis.Client
	caches  map[string]ICache
}

func (rr *redisClients) open(cfg *config.Config) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	rr.clients = make(map[string]*redis.Client)
	rr.caches = make(map[string]ICache)

	for name, c := range cfg.App.Middleware.Redis {
		if !c.Startup {
			continue
		}
		rr.clients[name] = redis.NewClient(redisOptions(&c))
	}

	for name, c := range cfg.App.Middleware.Cache {
		if c.BackendType != conftype.TypeCacheRedis {
			continue
		}
		client, ok := rr.clients[c.Name]
		if !ok {
			return fmt.Errorf("cache %q references unknown redis %q", name, c.Name)
		}
		rr.caches[name] = NewRedisCache(client)
	}

	return nil
}

func (rr *redisClients) get(name string) (*redis.Client, error) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	client, ok := rr.clients[name]
	if !ok {
		return nil, resourceError("redis", name, nil)
	}
	return client, nil
}

func (rr *redisClients) cache(name string) (ICache, error) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	cache, ok := rr.caches[name]
	if !ok {
		return nil, resourceError("cache", name, nil)
	}
	return cache, nil
}

func (rr *redisClients) close() error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	var errs []error
	for name, client := range rr.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close redis %q: %w", name, err))
		}
	}

	rr.clients = nil
	rr.caches = nil

	return errors.Join(errs...)
}

func (rr *redisClients) healthy(ctx context.Context) []error {
	type namedClient struct {
		name   string
		client *redis.Client
	}

	rr.mu.RLock()
	clients := make([]namedClient, 0, len(rr.clients))
	for name, client := range rr.clients {
		clients = append(clients, namedClient{name: name, client: client})
	}
	rr.mu.RUnlock()

	var errs []error
	for _, item := range clients {
		if err := item.client.WithContext(ctx).Ping().Err(); err != nil {
			errs = append(errs, fmt.Errorf("redis %q: %w", item.name, err))
		}
	}
	return errs
}

// Redis returns a configured Redis client by name.
func (s *Set) Redis(name string) (*redis.Client, error) {
	if s == nil {
		return nil, resourceError("redis", name, nil)
	}
	return s.redis.get(name)
}

// DefaultRedis returns the default Redis client.
//
// It returns nil when Redis is not configured. The error-returning variant
// is Redis(defaultName).
func (s *Set) DefaultRedis() *redis.Client {
	client, err := s.Redis(defaultName)
	if err != nil {
		return nil
	}
	return client
}

// Cache returns a configured cache by name.
func (s *Set) Cache(name string) (ICache, error) {
	if s == nil {
		return nil, resourceError("cache", name, nil)
	}
	return s.redis.cache(name)
}

// DefaultCache returns the default cache.
func (s *Set) DefaultCache() (ICache, error) {
	return s.Cache(defaultName)
}

// redisOptions converts the YAML-facing config into go-redis options.
func redisOptions(opt *conftype.Redis) *redis.Options {
	redisOptions := &redis.Options{
		Network:      opt.Network,
		Addr:         opt.Addr,
		Password:     opt.Password,
		DB:           opt.DB,
		PoolSize:     opt.PoolSize,
		MaxRetries:   opt.MaxRetries,
		MinIdleConns: opt.MinIdleConns,
	}
	if opt.MinRetryBackoff > 0 {
		redisOptions.MinRetryBackoff = opt.MinRetryBackoff * time.Millisecond
	}
	if opt.MaxRetryBackoff > 0 {
		redisOptions.MaxRetryBackoff = opt.MaxRetryBackoff * time.Millisecond
	}
	if opt.DialTimeout > 0 {
		redisOptions.DialTimeout = opt.DialTimeout * time.Millisecond
	}
	if opt.ReadTimeout > 0 {
		redisOptions.ReadTimeout = opt.ReadTimeout * time.Millisecond
	}
	if opt.WriteTimeout > 0 {
		redisOptions.WriteTimeout = opt.WriteTimeout * time.Millisecond
	}
	if opt.PoolTimeout > 0 {
		redisOptions.PoolTimeout = opt.PoolTimeout * time.Millisecond
	}
	if opt.MaxConnAge > 0 {
		redisOptions.MaxConnAge = opt.MaxConnAge * time.Millisecond
	}
	if opt.IdleTimeout > 0 {
		redisOptions.IdleTimeout = opt.IdleTimeout * time.Millisecond
	}
	if opt.IdleCheckFrequency > 0 {
		redisOptions.IdleCheckFrequency = opt.IdleCheckFrequency * time.Millisecond
	}
	return redisOptions
}
