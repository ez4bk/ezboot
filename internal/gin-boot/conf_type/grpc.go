package conf_type

import (
	"time"

	"github.com/ez4bk/ezboot/internal/gin-boot/util/custom"
)

// GRPCClient GRPCClient
type GRPCClient struct {
	// Naming if using etcd for naming
	Naming bool `yaml:"naming"`

	// Addr is the upstream gRPC address.
	Addr string `yaml:"addr"`

	TimeoutPerRequest custom.YAMLDuration `yaml:"timeout_per_request"`
	// WithWriteBufferSize determines how much data can be batched before doing a
	// write on the wire. The corresponding memory allocation for this buffer will
	// be twice the size to keep syscalls low. The default value for this buffer is
	// 32KB.
	//
	// Zero will disable the write buffer such that each write will be on underlying
	// connection. Note: A Send call may not directly translate to a write.
	WithWriteBufferSize string `yaml:"with_write_buffer_size"`

	// WithReadBufferSize lets you set the size of read buffer, this determines how
	// much data can be read at most for each read syscall.
	//
	// The default value for this buffer is 32KB. Zero will disable read buffer for
	// a connection so data framer can access the underlying conn directly.
	WithReadBufferSize string `yaml:"with_read_buffer_size"`

	// WithInitialWindowSize returns a DialOption which sets the value for initial
	// window size on a stream. The lower bound for window size is 64K and any value
	// smaller than that will be ignored.
	WithInitialWindowSize int32 `yaml:"with_initial_window_size"`
	// WithInitialConnWindowSize returns a DialOption which sets the value for
	// initial window size on a connection. The lower bound for window size is 64K
	// and any value smaller than that will be ignored.
	WithInitialConnWindowSize int32 `yaml:"with_initial_conn_window_size"`
	// WithMaxCallRecvMsgSize sets the maximum message size the client can
	// receive. It is applied with grpc.WithDefaultCallOptions and
	// grpc.MaxCallRecvMsgSize.
	WithMaxCallRecvMsgSize int `yaml:"with_max_call_recv_msg_size"`
	// WithBalancerName sets the balancer that the ClientConn will be initialized
	// with. Balancer registered with balancerName will be used. This function
	// panics if no balancer was registered by balancerName.
	//
	// The balancer cannot be overridden by balancer option specified by service
	// config.
	//
	// This is an EXPERIMENTAL API.
	WithBalancerName string `yaml:"with_balancer_name"`
	// ConnectParams configures gRPC connection backoff and the minimum connect
	// timeout. It maps directly to grpc.ConnectParams.
	ConnectParams *ConnectParams `yaml:"connect_params"`
	// WithInsecure returns a DialOption which disables transport security for this
	// ClientConn. Note that transport security is required unless WithInsecure is
	// set.
	WithInsecure bool `yaml:"with_insecure"`
	// IdleTimeout configures how long a connection may be idle before the client
	// closes it. It maps to grpc.WithIdleTimeout. The value is interpreted as
	// milliseconds.
	IdleTimeout time.Duration `yaml:"idle_timeout"`
	// DefaultServiceConfig is a JSON service config applied to the connection.
	// It maps to grpc.WithDefaultServiceConfig and can be used for load balancing
	// and retry policies.
	DefaultServiceConfig string `yaml:"default_service_config"`
	// MaxCallSendMsgSize sets the maximum message size the client can send.
	// It maps to grpc.MaxCallSendMsgSize through grpc.WithDefaultCallOptions.
	MaxCallSendMsgSize int `yaml:"max_call_send_msg_size"`
	// WithUserAgent returns a DialOption that specifies a user agent string for all
	// the RPCs.
	WithUserAgent string `yaml:"with_user_agent"`
	// WithKeepaliveParams returns a DialOption that specifies keepalive parameters
	// for the client transport.
	// WithKeepaliveParams
	// After a duration of this time if the client doesn't see any activity it
	// pings the server to see if the transport is still alive.
	// Time time.Duration // The current default value is infinity.
	// After having pinged for keepalive check, the client waits for a duration
	// of Timeout and if no activity is seen even after that the connection is
	// closed.
	// Timeout time.Duration // The current default value is 20 seconds.
	// If true, client sends keepalive pings even with no active RPCs. If false,
	// when there are no active RPCs, Time and Timeout will be ignored and no
	// keepalive pings will be sent.
	// PermitWithoutStream bool // false by default.

	// WithAuthority returns a DialOption that specifies the value to be used as the
	// :authority pseudo-header. This value only works with WithInsecure and has no
	// effect if TransportCredentials are present.
	WithAuthority string `yaml:"with_authority"`
	// WithChannelzParentID returns a DialOption that specifies the channelz ID of
	// current ClientConn's parent. This function is used in nested channel creation
	// (e.g. grpclb dial).
	WithChannelzParentID int64 `yaml:"with_channelz_parent_id"`
	// WithDisableServiceConfig returns a DialOption that causes grpc to ignore any
	// service config provided by the resolver and provides a hint to the resolver
	// to not fetch service configs.
	WithDisableServiceConfig bool `yaml:"with_disable_service_config"`
	// WithDisableRetry returns a DialOption that disables retries, even if the
	// service config enables them.  This does not impact transparent retries, which
	// will happen automatically if no data is written to the wire or if the RPC is
	// unprocessed by the remote server.
	//
	// Retry support is currently disabled by default, but will be enabled by
	// default in the future.  Until then, it may be enabled by setting the
	// environment variable "GRPC_GO_RETRY" to "on".
	//
	// This API is EXPERIMENTAL.
	WithDisableRetry bool `yaml:"with_disable_retry"`
	// WithMaxHeaderListSize returns a DialOption that specifies the maximum
	// (uncompressed) size of header list that the client is prepared to accept.
	WithMaxHeaderListSize uint32 `yaml:"with_max_header_list_size"`

	// ClientParameters is used to set keepalive parameters on the client-side.
	// These configure how the client will actively probe to notice when a
	// connection is broken and send pings so intermediaries will be aware of the
	// liveness of the connection. Make sure these parameters are set in
	// coordination with the keepalive policy on the server, as incompatible
	// settings can result in closing of connection.

	// After a duration of this time if the client doesn't see any activity it
	// pings the server to see if the transport is still alive.
	// The current default value is infinity.
	KeepaliveTime time.Duration `yaml:"keepalive_time"`
	// After having pinged for keepalive check, the client waits for a duration
	// of Timeout and if no activity is seen even after that the connection is
	// closed.
	// The current default value is 20 seconds.
	KeepaliveTimeout time.Duration `yaml:"keepalive_timeout"`
	// If true, client sends keepalive pings even with no active RPCs. If false,
	// when there are no active RPCs, Time and Timeout will be ignored and no
	// keepalive pings will be sent.
	// false by default.
	KeepalivePermitWithoutStream bool `yaml:"keepalive_permit_without_stream"`
}

// GRPCServer GRPCServer
type GRPCServer struct {
	Startup_ bool   `yaml:"_startup"`
	Addr     string `yaml:"addr"`

	// WriteBufferSize determines how much data can be batched before doing a write on the wire.
	// The corresponding memory allocation for this buffer will be twice the size to keep syscalls low.
	// The default value for this buffer is 32KB.
	// Zero will disable the write buffer such that each write will be on underlying connection.
	// Note: A Send call may not directly translate to a write.
	// int
	WriteBufferSize string `yaml:"write_buffer_size"`

	// ReadBufferSize lets you set the size of read buffer, this determines how much data can be read at most
	// for one read syscall.
	// The default value for this buffer is 32KB.
	// Zero will disable read buffer for a connection so data framer can access the underlying
	// conn directly.
	// int
	ReadBufferSize string `yaml:"read_buffer_size"`

	// InitialWindowSize returns a ServerOption that sets window size for stream.
	// The lower bound for window size is 64K and any value smaller than that will be ignored.
	// int32
	InitialWindowSize int32 `yaml:"initial_window_size"`

	// InitialConnWindowSize returns a ServerOption that sets window size for a connection.
	// The lower bound for window size is 64K and any value smaller than that will be ignored.
	InitialConnWindowSize int32 `yaml:"initial_conn_window_size"`

	/*
		// KeepaliveParams returns a ServerOption that sets keepalive and max-age parameters for the server.
		func KeepaliveParams(kp keepalive.ServerParameters) ServerOption {
		// KeepaliveEnforcementPolicy returns a ServerOption that sets keepalive enforcement policy for the server.
		func KeepaliveEnforcementPolicy(kep keepalive.EnforcementPolicy) ServerOption {
	*/

	// MaxRecvMsgSize returns a ServerOption to set the max message size in bytes the server can receive.
	// If this is not set, gRPC uses the default 4MB.
	// int
	MaxRecvMsgSize string `yaml:"max_recv_msg_size"`

	// MaxSendMsgSize returns a ServerOption to set the max message size in bytes the server can send.
	// If this is not set, gRPC uses the default 4MB.
	// int
	MaxSendMsgSize string `yaml:"max_send_msg_size"`

	// MaxConcurrentStreams returns a ServerOption that will apply a limit on the number
	// of concurrent streams to each ServerTransport.
	MaxConcurrentStreams uint32 `yaml:"max_concurrent_streams"`

	// ConnectionTimeout returns a ServerOption that sets the timeout for
	// connection establishment (up to and including HTTP/2 handshaking) for all
	// new connections.  If this is not set, the default is 120 seconds.  A zero or
	// negative value will result in an immediate timeout.
	//
	// This API is EXPERIMENTAL.
	// d time.Duration
	ConnectionTimeout time.Duration `yaml:"connection_timeout"`

	// MaxHeaderListSize returns a ServerOption that sets the max (uncompressed) size
	// of header list that the server is prepared to accept.
	MaxHeaderListSize uint32 `yaml:"max_header_list_size"`

	// Keepalive ServerParameters is used to set keepalive and max-age parameters on the
	// server-side.

	// MaxConnectionIdle is a duration for the amount of time after which an
	// idle connection would be closed by sending a GoAway. Idleness duration is
	// defined since the most recent time the number of outstanding RPCs became
	// zero or the connection establishment.
	// The current default value is infinity.
	KeepaliveMaxConnectionIdle time.Duration `yaml:"keepalive_max_connection_idle"`

	// MaxConnectionAge is a duration for the maximum amount of time a
	// connection may exist before it will be closed by sending a GoAway. A
	// random jitter of +/-10% will be added to MaxConnectionAge to spread out
	// connection storms.
	// The current default value is infinity.
	KeepaliveMaxConnectionAge time.Duration `yaml:"keepalive_max_connection_age"`

	// MaxConnectinoAgeGrace is an additive period after MaxConnectionAge after
	// which the connection will be forcibly closed.
	// The current default value is infinity.
	KeepaliveMaxConnectionAgeGrace time.Duration `yaml:"keepalive_max_connection_age_grace"`

	// After a duration of this time if the server doesn't see any activity it
	// pings the client to see if the transport is still alive.
	// The current default value is 2 hours.
	KeepaliveTime time.Duration `yaml:"keepalive_time"`

	// After having pinged for keepalive check, the server waits for a duration
	// of Timeout and if no activity is seen even after that the connection is
	// closed.
	// The current default value is 20 seconds.
	KeepaliveTimeout time.Duration `yaml:"keepalive_timeout"`

	// EnforcementPolicy is used to set keepalive enforcement policy on the
	// server-side. Server will close connection with a client that violates this
	// policy.

	// MinTime is the minimum amount of time a client should wait before sending
	// a keepalive ping.
	// The current default value is 5 minutes.
	KeepaliveEnforcementPolicyMinTime time.Duration `yaml:"keepalive_enforcement_policy_min_time"`
	// If true, server allows keepalive pings even when there are no active
	// streams(RPCs). If false, and client sends ping when there are no active
	// streams, server will send GOAWAY and close the connection.
	// false by default.
	KeepaliveEnforcementPolicyPermitWithoutStream bool `yaml:"keepalive_enforcement_policy_permit_without_stream"`
}

// ConnectParams configures gRPC connection backoff and minimum connect timeout.
// Durations are interpreted as milliseconds.
type ConnectParams struct {
	Backoff           BackoffConfig `yaml:"backoff"`
	MinConnectTimeout time.Duration `yaml:"min_connect_timeout"`
}

// BackoffConfig configures exponential reconnect backoff.
// Durations are interpreted as milliseconds.
type BackoffConfig struct {
	BaseDelay  time.Duration `yaml:"base_delay"`
	Multiplier float64       `yaml:"multiplier"`
	Jitter     float64       `yaml:"jitter"`
	MaxDelay   time.Duration `yaml:"max_delay"`
}

// GRPCClients maps logical client names to their configuration.
//
// The YAML map key is the client name used by GetClient and by the
// grpc_client_inject tag.
type GRPCClients map[string]*GRPCClient

// GRPC GRPC
type GRPC struct {
	Client GRPCClients `yaml:"client"`
	Server struct {
		Default GRPCServer `yaml:"default"`
	} `yaml:"server"`
}
