package infra

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/ez4bk/ezboot"
	"github.com/google/wire"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
)

type etcdClient struct {
	mu sync.RWMutex

	client  *clientv3.Client
	polling bool
}

func (e *etcdClient) open(cfg *config.Config) error {
	conf := &cfg.App.Middleware.Etcd
	if !conf.Startup {
		return nil
	}

	clientConf := clientv3.Config{
		Endpoints:   cfg.App.Middleware.Etcd.Endpoints,
		DialTimeout: 20 * time.Second,
	}
	if conf.TLS {
		tlsInfo := transport.TLSInfo{
			CertFile:      conf.CertFile,
			KeyFile:       conf.KeyFile,
			TrustedCAFile: conf.CAFile,
		}
		tls, err := tlsInfo.ClientConfig()
		if err != nil {
			return resourceError("etcd", defaultName, err)
		}
		clientConf.TLS = tls
	}

	client, err := clientv3.New(clientConf)
	if err != nil {
		return resourceError("etcd", defaultName, err)
	}

	e.mu.Lock()
	e.client = client
	e.polling = conf.Polling
	e.mu.Unlock()

	return nil
}

func (e *etcdClient) get() (*clientv3.Client, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.client == nil {
		return nil, resourceError("etcd", defaultName, nil)
	}
	return e.client, nil
}

func (e *etcdClient) isPolling() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.polling
}

func (e *etcdClient) close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.client == nil {
		return nil
	}
	err := e.client.Close()
	e.client = nil
	return err
}

func (e *etcdClient) healthy(ctx context.Context) []error {
	e.mu.RLock()
	client := e.client
	e.mu.RUnlock()

	if client == nil {
		return nil
	}

	endpoints := client.Endpoints()
	if len(endpoints) == 0 {
		return []error{fmt.Errorf("etcd: no endpoints configured")}
	}

	var errs []error
	for _, endpoint := range endpoints {
		if _, err := client.Status(ctx, endpoint); err != nil {
			errs = append(errs, fmt.Errorf("etcd endpoint %q: %w", endpoint, err))
		}
	}
	return errs
}

// Etcd returns the configured etcd client.
func (s *Etcd) Etcd() (*clientv3.Client, error) {
	if s == nil {
		return nil, resourceError("etcd", defaultName, nil)
	}
	return s.etcdClient.get()
}

// EtcdPolling reports whether remote config should use polling instead of a
// watch stream.
func (s *Etcd) EtcdPolling() bool {
	if s == nil {
		return false
	}
	return s.etcdClient.isPolling()
}

// Etcd 是 etcd 插件。远程配置中心也挂在这个插件上。
type Etcd struct {
	ezboot.PluginBase

	conf *config.Config
	etcdClient
}

// EtcdProviderSet 提供插件，以及默认客户端。
var EtcdProviderSet = wire.NewSet(NewEtcd, ProvideDefaultEtcd)

// NewEtcd 创建插件并建立客户端。
func NewEtcd(cfg *config.Config) (*Etcd, error) {
	if cfg == nil {
		return nil, errors.New("infra: etcd requires a config")
	}
	if len(cfg.App.Middleware.Etcd.Endpoints) == 0 {
		return nil, errors.New("infra: etcd plugin is enabled but no endpoint is configured")
	}

	e := &Etcd{conf: cfg}
	if err := e.open(cfg); err != nil {
		return nil, err
	}

	return e, nil
}

// Name 返回插件名。
func (e *Etcd) Name() string { return "etcd" }

// Start 阻塞到 ctx 结束。
func (e *Etcd) Start(ctx context.Context) error {
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

// ProvideDefaultEtcd 提供默认 etcd 客户端。
func ProvideDefaultEtcd(e *Etcd) (*clientv3.Client, error) {
	return e.Etcd()
}
