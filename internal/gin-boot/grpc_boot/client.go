package grpc_boot

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"sync"
	"time"

	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"go.elastic.co/apm/module/apmgrpc"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"github.com/ez4bk/ezboot/internal/gin-boot/conf_type"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware"
	"github.com/ez4bk/ezboot/internal/gin-boot/util"
	"github.com/ez4bk/ezboot/internal/logging"
)

const clientInjectTag = "grpc_client_inject"
const defaultClientTimeoutPerRequest = time.Second * 5
const clientPerReqTimeout = "cli_req_tmout"

var passThroughMetadata = []string{middleware.UserKey, middleware.TraceID}

var (
	defaultKeepAliveParams = keepalive.ClientParameters{
		Time:                time.Duration(math.MaxInt64),
		Timeout:             20 * time.Second,
		PermitWithoutStream: false,
	}

	// SomeServiceClient => func(*grpc.ClientConn) SomeServiceClient
	registered = map[reflect.Type]*reflect.Value{}
)

// ErrClientsClosed is returned when a client is requested after the pool has
// been closed.
var ErrClientsClosed = errors.New("grpc: client pool is closed")

var (
	clientsMu     sync.RWMutex
	clients       = map[string]*grpc.ClientConn{}
	confClients   = &conf_type.GRPCClients{}
	clientsClosed bool
	clientGroup   singleflight.Group
)

func closeClientConns(conns map[string]*grpc.ClientConn) error {
	var errs []error
	for name, conn := range conns {
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close grpc client %q: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

// InjectAllClient InjectAllClient
func InjectAllClient(v any) {
	log := logging.Default().With("function", "inject_grpc_client")
	if v == nil {
		log.Panic("unable inject client to nil value")
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		log.Panic("unable bind to non-pointer value")
	}

	if rv = rv.Elem(); rv.Kind() != reflect.Struct {
		log.Panic("unsupported type, use struct instead")
	}

	if err := injectClientToStruct(rv); err != nil {
		log.Panic(err)
	}
}

// injectClientToStruct 将对应的 GRPC 客户端注入到结构体中
func injectClientToStruct(v reflect.Value) error {
	rt := v.Type()

	for i := 0; i < v.NumField(); i++ {
		if fv := v.Field(i); fv.CanSet() && fv.CanAddr() {
			ft := fv.Type()

			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
				if fv.IsNil() {
					fv.Set(reflect.New(ft))
				}
				fv = fv.Elem()
			}

			if fv.Kind() == reflect.Struct {
				if err := injectClientToStruct(fv); err != nil {
					return err
				}
				continue
			}

			if fv.Kind() == reflect.Interface {
				if tag := rt.Field(i).Tag.Get(clientInjectTag); len(tag) != 0 {
					if construct, ok := registered[ft]; ok {
						conn, err := GetClient(tag)
						if err != nil {
							return err
						}

						results := construct.Call([]reflect.Value{reflect.ValueOf(conn)})
						fv.Set(results[0]) // it's okay
					}
				}
				continue
			}
		}
	}

	return nil
}

// RegisterClient : RegisterClient("server_kind", func(*grpc.ClientConn) SomeServiceClient)
func RegisterClient(serverKind string, newFunc any) {
	logger := logging.Default().With("error_when_RegisterClient", serverKind)

	tNewFunc := reflect.TypeOf(newFunc)
	vNewFunc := reflect.ValueOf(newFunc)
	if tNewFunc.Kind() != reflect.Func {
		logger.Fatal("newFunc should a Func")
	}
	if tNewFunc.NumIn() != 1 {
		logger.Fatal("newFunc should have/only have one In parameter")
	}
	if tNewFunc.NumOut() != 1 {
		logger.Fatal("newFunc should have/only have one Out result")
	}

	clientType := tNewFunc.Out(0)
	registered[clientType] = &vNewFunc
}

// InitClient replaces the process-level client pool.
// The YAML map key is the client name.
func InitClient(conf *conf_type.GRPCClients) {
	clientsMu.Lock()
	old := clients
	clients = make(map[string]*grpc.ClientConn)
	confClients = conf
	clientsClosed = false
	clientsMu.Unlock()

	_ = closeClientConns(old)
}

// GetClient returns a cached connection from the process-level pool.
func GetClient(name string) (*grpc.ClientConn, error) {
	clientsMu.RLock()
	if clientsClosed {
		clientsMu.RUnlock()
		return nil, ErrClientsClosed
	}
	if conn, ok := clients[name]; ok {
		clientsMu.RUnlock()
		return conn, nil
	}
	conf := confClients
	clientsMu.RUnlock()

	if conf == nil {
		return nil, errors.New("grpc: client config is nil")
	}

	value, err, _ := clientGroup.Do(name, func() (any, error) {
		clientsMu.Lock()
		if clientsClosed {
			clientsMu.Unlock()
			return nil, ErrClientsClosed
		}
		if conn, ok := clients[name]; ok {
			clientsMu.Unlock()
			return conn, nil
		}
		cfg, ok := (*conf)[name]
		clientsMu.Unlock()
		if !ok || cfg == nil {
			return nil, fmt.Errorf("not defined in config: grpc client: %v", name)
		}

		conn, err := NewClient(cfg.Addr, cfg.TimeoutPerRequest.Duration, GetDialOptions(cfg)...)
		if err != nil {
			return nil, err
		}

		clientsMu.Lock()
		if clientsClosed {
			clientsMu.Unlock()
			_ = conn.Close()
			return nil, ErrClientsClosed
		}
		clients[name] = conn
		clientsMu.Unlock()
		return conn, nil
	})
	if err != nil {
		return nil, err
	}
	conn, _ := value.(*grpc.ClientConn)
	return conn, nil
}

// CloseClients closes and clears the process-level client pool.
func CloseClients() error {
	clientsMu.Lock()
	if clientsClosed {
		clientsMu.Unlock()
		return nil
	}
	clientsClosed = true
	old := clients
	clients = make(map[string]*grpc.ClientConn)
	clientsMu.Unlock()

	return closeClientConns(old)
}

// NewClient creates a standalone connection that is not managed by a pool.
func NewClient(addr string, timeout time.Duration, dialOptions ...grpc.DialOption) (*grpc.ClientConn, error) {
	return dial(addr, timeout, dialOptions...)
}

func copyRequestContextInterceptor(copyFields []string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		outgoingMD, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			outgoingMD = metadata.New(map[string]string{})
		}

		if incomingMD, ok := metadata.FromIncomingContext(ctx); ok {
			for _, field := range copyFields {

				// already set by microservice, skip
				if len(outgoingMD.Get(field)) > 0 {
					continue
				}

				// not set by incoming context, skip
				if len(incomingMD.Get(field)) == 0 {
					continue
				}

				outgoingMD.Set(field, incomingMD.Get(field)...)
			}
			ctx = metadata.NewOutgoingContext(ctx, outgoingMD)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func requestTimeoutClientInterceptor(timeoutPerRequest time.Duration) grpc.UnaryClientInterceptor {
	if timeoutPerRequest == 0 {
		timeoutPerRequest = defaultClientTimeoutPerRequest
	}
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctxTimeout, canceler := context.WithTimeout(ctx, timeoutPerRequest)
		ctxTmout := util.AppendToOutgoingContext(ctxTimeout, clientPerReqTimeout, timeoutPerRequest.String())
		defer canceler()
		return invoker(ctxTmout, method, req, reply, cc, opts...)
	}
}

func dial(addr string, timeoutPerRequest time.Duration, dialOptions ...grpc.DialOption) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, append(
		dialOptions,
		grpc.WithChainUnaryInterceptor(
			apmgrpc.NewUnaryClientInterceptor(),
			grpcprometheus.UnaryClientInterceptor,
			middleware.GRPCEnvClientInterceptor(),
			requestTimeoutClientInterceptor(timeoutPerRequest),
			copyRequestContextInterceptor(passThroughMetadata),
		),
		grpc.WithChainStreamInterceptor(),
	)...)
}

// GetDialOptions 根据配置获取
func GetDialOptions(options *conf_type.GRPCClient) []grpc.DialOption {
	var dialOptions []grpc.DialOption

	if options.WithWriteBufferSize != "" {
		writeBufferSize, err := strconv.Atoi(options.WithWriteBufferSize)
		if err != nil {
			util.ChkErr(errors.New("rpc with_write_buffer_size config error"))
		}
		dialOptions = append(dialOptions, grpc.WithWriteBufferSize(writeBufferSize))
	}

	if options.WithReadBufferSize != "" {
		readBufferSize, err := strconv.Atoi(options.WithReadBufferSize)
		if err != nil {
			util.ChkErr(errors.New("rpc with_read_buffer_size config error"))
		}
		dialOptions = append(dialOptions, grpc.WithReadBufferSize(readBufferSize))
	}

	if options.WithInitialWindowSize > 0 {
		dialOptions = append(dialOptions, grpc.WithInitialWindowSize(options.WithInitialWindowSize))
	}

	if options.WithInitialConnWindowSize > 0 {
		dialOptions = append(dialOptions, grpc.WithInitialConnWindowSize(options.WithInitialConnWindowSize))
	}

	var callOptions []grpc.CallOption
	if options.WithMaxCallRecvMsgSize > 0 {
		callOptions = append(callOptions, grpc.MaxCallRecvMsgSize(options.WithMaxCallRecvMsgSize))
	}
	if options.MaxCallSendMsgSize > 0 {
		callOptions = append(callOptions, grpc.MaxCallSendMsgSize(options.MaxCallSendMsgSize))
	}
	if len(callOptions) > 0 {
		dialOptions = append(dialOptions, grpc.WithDefaultCallOptions(callOptions...))
	}

	if options.ConnectParams != nil {
		connectParams := grpc.ConnectParams{Backoff: backoff.DefaultConfig}
		if options.ConnectParams.Backoff.BaseDelay > 0 {
			connectParams.Backoff.BaseDelay = options.ConnectParams.Backoff.BaseDelay * time.Millisecond
		}
		if options.ConnectParams.Backoff.Multiplier > 0 {
			connectParams.Backoff.Multiplier = options.ConnectParams.Backoff.Multiplier
		}
		if options.ConnectParams.Backoff.Jitter > 0 {
			connectParams.Backoff.Jitter = options.ConnectParams.Backoff.Jitter
		}
		if options.ConnectParams.Backoff.MaxDelay > 0 {
			connectParams.Backoff.MaxDelay = options.ConnectParams.Backoff.MaxDelay * time.Millisecond
		}
		if options.ConnectParams.MinConnectTimeout > 0 {
			connectParams.MinConnectTimeout = options.ConnectParams.MinConnectTimeout * time.Millisecond
		}
		dialOptions = append(dialOptions, grpc.WithConnectParams(connectParams))
	}

	if options.IdleTimeout > 0 {
		dialOptions = append(dialOptions, grpc.WithIdleTimeout(options.IdleTimeout*time.Millisecond))
	}

	if options.WithInsecure == true {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if options.WithUserAgent != "" {
		dialOptions = append(dialOptions, grpc.WithUserAgent(options.WithUserAgent))
	}

	if options.WithAuthority != "" {
		dialOptions = append(dialOptions, grpc.WithAuthority(options.WithAuthority))
	}

	if options.WithDisableServiceConfig == true {
		dialOptions = append(dialOptions, grpc.WithDisableServiceConfig())
	}

	if options.WithDisableRetry == true {
		dialOptions = append(dialOptions, grpc.WithDisableRetry())
	}

	if options.WithMaxHeaderListSize > 0 {
		dialOptions = append(dialOptions, grpc.WithMaxHeaderListSize(options.WithMaxHeaderListSize))
	}

	if options.DefaultServiceConfig != "" {
		dialOptions = append(dialOptions, grpc.WithDefaultServiceConfig(options.DefaultServiceConfig))
	}

	// keepalive ClientParameters
	if options.KeepaliveTime > 0 || options.KeepaliveTimeout > 0 || options.KeepalivePermitWithoutStream != false {
		keepaliveParams := defaultKeepAliveParams
		if options.KeepaliveTime > 0 {
			keepaliveParams.Time = options.KeepaliveTime * time.Millisecond
		}
		if options.KeepaliveTimeout > 0 {
			keepaliveParams.Timeout = options.KeepaliveTimeout * time.Millisecond
		}
		if options.KeepalivePermitWithoutStream != false {
			keepaliveParams.PermitWithoutStream = options.KeepalivePermitWithoutStream
		}
		dialOptions = append(dialOptions, grpc.WithKeepaliveParams(keepaliveParams))
	}

	return dialOptions
}
