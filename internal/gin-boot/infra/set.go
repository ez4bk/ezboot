package infra

import (
	"context"
	"errors"
	"fmt"
	"sync"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/util"
	"github.com/ez4bk/ezboot/internal/logging"
)

// ErrResourceUnavailable is returned when a configured resource cannot be
// opened, or when a caller asks for a resource that was not configured.
var ErrResourceUnavailable = errors.New("ezboot: resource unavailable")

// ResourceError describes which resource failed and why.
//
// Callers can use errors.Is(err, ErrResourceUnavailable) to distinguish a
// missing/unavailable dependency from a business error.
type ResourceError struct {
	Kind string
	Name string

	Err error
}

func (e *ResourceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("ezboot: %s %q unavailable: %v", e.Kind, e.Name, e.Err)
	}
	return fmt.Sprintf("ezboot: %s %q unavailable", e.Kind, e.Name)
}

func (e *ResourceError) Unwrap() error {
	return e.Err
}

func (e *ResourceError) Is(target error) bool {
	return target == ErrResourceUnavailable
}

func resourceError(kind, name string, err error) error {
	return &ResourceError{Kind: kind, Name: name, Err: err}
}

// Set owns the process-level infrastructure clients: databases, Redis,
// message brokers and search clients.
//
// The interface is intentionally small:
//
//	DB/MySQL/SQLite/Pgsql   -> typed handles or an explicit error
//	Redis/Cache             -> clients or an explicit error
//	Kafka/RabbitMQ          -> clients or an explicit error
//	Elasticsearch           -> client or an explicit error
//	Etcd                    -> client or an explicit error
//	Close(ctx)              -> idempotent, ordered shutdown
//	Healthy(ctx)            -> dependency health for readiness endpoints
//
// Set never returns a nil handle for a missing resource. Prefer injecting
// narrow client types (or *Set when a module truly needs several clients)
// into the modules that need them, instead of reading the package-level
// Instance.
type Set struct {
	conf   *config.Config
	logger *logging.Logger

	db       dbConns
	redis    redisClients
	kafka    kafkaClients
	rabbitMQ rabbitMQ
	search   searchClients
	etcd     etcdClient

	tracerProvider *tracesdk.TracerProvider

	closeOnce sync.Once
	closeErr  error
}

// Instance is the process-level default Set used by the legacy package-level
// helpers (with.DefaultSession, boot.Infra, ...).
//
// New code should receive explicit dependencies instead of reading this
// variable. It remains only to keep v0.4.x call sites working during the
// migration.
var Instance = &Set{}

// Default returns the process-level default Set.
func Default() *Set {
	return Instance
}

// SetDefault replaces the process-level default. It is called by the
// framework bootstrap and should not be used by application code.
func SetDefault(r *Set) {
	if r == nil {
		return
	}
	Instance = r
}

// Open builds a Set from config. It does not publish the process-default
// (call SetDefault from the composition root), and returns every
// initialization error to the caller.
func Open(cfg *config.Config, logger *logging.Logger) (*Set, error) {
	if cfg == nil {
		return nil, errors.New("ezboot: config is nil")
	}
	if logger == nil {
		logger = logging.Default()
	}

	r := &Set{conf: cfg, logger: logger}

	steps := []func() error{
		func() error { return r.db.open(cfg, logger) },
		func() error { return r.redis.open(cfg) },
		func() error { return r.etcd.open(cfg) },
		func() error { return r.kafka.open(cfg) },
		func() error { return r.rabbitMQ.open(cfg, logger) },
		func() error { return r.search.open(cfg) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			// Close whatever was already opened before returning. The
			// original error is the one callers need; close errors are
			// secondary but still joined so they are not silently lost.
			return nil, errors.Join(err, r.Close(context.Background()))
		}
	}

	return r, nil
}

// Init builds a Set, publishes it as the process default, and panics on
// failure. It is the compatibility entry point used by the old bootstrap.
//
// Deprecated: use Open (and SetDefault only inside the composition root).
func Init(cfg *config.Config, logger *logging.Logger) *Set {
	r, err := Open(cfg, logger)
	util.ChkErr(err)
	SetDefault(r)

	return r
}

// Shutdown closes the process-level default Set.
func Shutdown() error {
	return Default().Close(context.Background())
}

// ShutdownContext is Shutdown with a caller-provided deadline. Use it from a
// graceful-shutdown path so a stuck dependency cannot block process exit.
func ShutdownContext(ctx context.Context) error {
	return Default().Close(ctx)
}

// Conf returns the config this Set was opened with.
func (s *Set) Conf() *config.Config {
	if s == nil {
		return nil
	}
	return s.conf
}

// Logger returns the logger this Set was opened with.
func (s *Set) Logger() *logging.Logger {
	if s == nil {
		return nil
	}
	return s.logger
}

// SetTracerProvider attaches the process tracer provider so Set.Close can
// flush it last. It is called once by the composition root.
func (s *Set) SetTracerProvider(tp *tracesdk.TracerProvider) {
	if s != nil {
		s.tracerProvider = tp
	}
}

// Close releases every owned client exactly once. It is safe to call on a
// nil or partially initialized Set value.
//
// Clients are closed in reverse dependency order (RabbitMQ/Kafka, search,
// etcd, Redis, then databases) and the tracer provider is flushed last so
// spans emitted during shutdown are not lost.
func (s *Set) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.closeOnce.Do(func() {
		var errs []error

		errs = append(errs, s.rabbitMQ.close())
		errs = append(errs, s.kafka.close())
		errs = append(errs, s.search.close())
		errs = append(errs, s.etcd.close())
		errs = append(errs, s.redis.close())
		errs = append(errs, s.db.close())

		if s.tracerProvider != nil {
			errs = append(errs, s.tracerProvider.Shutdown(ctx))
		}

		s.closeErr = errors.Join(errs...)
	})

	return s.closeErr
}

// Healthy checks the dependencies that can be probed cheaply. It is intended
// for readiness endpoints; it must not be used as a liveness check.
func (s *Set) Healthy(ctx context.Context) error {
	if s == nil {
		return errors.New("ezboot: infrastructure is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var errs []error
	errs = append(errs, s.db.healthy(ctx)...)
	errs = append(errs, s.redis.healthy(ctx)...)

	return errors.Join(errs...)
}
