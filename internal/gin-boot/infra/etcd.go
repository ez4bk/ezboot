package infra

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"

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
func (s *Set) Etcd() (*clientv3.Client, error) {
	if s == nil {
		return nil, resourceError("etcd", defaultName, nil)
	}
	return s.etcd.get()
}

// EtcdPolling reports whether remote config should use polling instead of a
// watch stream.
func (s *Set) EtcdPolling() bool {
	if s == nil {
		return false
	}
	return s.etcd.isPolling()
}
