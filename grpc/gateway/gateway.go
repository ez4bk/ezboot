package gateway

import (
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ez4bk/ezboot/grpc/gateway/multipart"
)

// Handler 增强GRPC网关实例
type Handler struct {
	mux      *runtime.ServeMux
	endpoint string

	once       sync.Once
	dialOpts   []grpc.DialOption
	headers    map[string]bool
	registers  []EndpointRegister
	annotators []Annotator

	muxOptions []runtime.ServeMuxOption
	muxRoutes  []func(r *mux.Router) error

	errHandlers         []ErrorHandler
	routingErrorHandler []RoutingErrorHandler
}

// Setup 初始化网关端点和相应路由
func (gh *Handler) Setup(ctx context.Context) (hh http.Handler, err error) {
	gh.once.Do(func() {
		for _, register := range gh.registers {
			if err = register(ctx, gh.mux, gh.endpoint, gh.dialOpts); err != nil {
				return
			}
		}
	})
	if err != nil {
		return
	}

	hh = gh.mux
	if len(gh.muxRoutes) != 0 {
		r := mux.NewRouter()
		for _, route := range gh.muxRoutes {
			if err = route(r); err != nil {
				return nil, err
			}
		}
		r.PathPrefix("/").Handler(gh.mux)
		r.NotFoundHandler = r.NewRoute().BuildOnly().Handler(gh.mux).GetHandler()
		hh = r
	}

	return
}

// withOutgoingHeaderMatcher 用于限制哪些HttpHeader可以从网关透传出去
func (gh *Handler) withOutgoingHeaderMatcher(header string) (string, bool) {
	if v, ok := gh.headers[header]; v && ok {
		return header, true
	}
	return runtime.DefaultHeaderMatcher(header)
}

// withMetadataAnnotator 将 Http 请求中的头转换为 gRPC 的元数据
func (gh *Handler) withMetadataAnnotator(ctx context.Context, req *http.Request) metadata.MD {
	md := make(metadata.MD, 4)
	for _, annotator := range gh.annotators {
		annotator(ctx, req, md)
	}

	return md
}

// withErrorHandler 后端服务发生错误时的异常处理函数
func (gh *Handler) withErrorHandler(ctx context.Context, mux *runtime.ServeMux,
	m runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {

	for _, handler := range gh.errHandlers {
		if hErr := handler(ctx, mux, m, w, r, err); hErr != nil {
			err = hErr
		}
	}

	runtime.DefaultHTTPErrorHandler(ctx, mux, m, w, r, err)
}

// withRoutingErrorHandler 路由错误配置错误的处理函数
func (gh *Handler) withRoutingErrorHandler(ctx context.Context, mux *runtime.ServeMux, m runtime.Marshaler, w http.ResponseWriter, r *http.Request, code int) {
	for _, handler := range gh.routingErrorHandler {
		if handler(ctx, mux, m, w, r, code) {
			return
		}
	}

	routineErrorHandler(ctx, mux, m, w, r, code)
}

// HandlerOption 网关服务额外配置对象
type HandlerOption func(gh *Handler) error

// WithHandlerOutgoingHeader 设置可以透传出网关的HTTP头
func WithHandlerOutgoingHeader(headers ...string) HandlerOption {
	return func(gh *Handler) error {
		for _, header := range headers {
			gh.headers[header] = true
		}
		return nil
	}
}

// EndpointRegister 用于在网关上注册端点并将请求转发到GRPC服务中
type EndpointRegister func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

// WithHandlerEndpointRegister 在网关上注册服务端点
func WithHandlerEndpointRegister(register EndpointRegister) HandlerOption {
	return func(gh *Handler) error {
		gh.registers = append(gh.registers, register)
		return nil
	}
}

// WithHandlerBaseEndpoint 设置网关的端点前缀
func WithHandlerBaseEndpoint(endpoint string) HandlerOption {
	return func(gh *Handler) error {
		gh.endpoint = endpoint
		return nil
	}
}

// WithHandlerDialOptions 在注册端点时的额外注册参数
func WithHandlerDialOptions(options ...grpc.DialOption) HandlerOption {
	return func(gh *Handler) error {
		gh.dialOpts = append(gh.dialOpts, options...)
		return nil
	}
}

// Annotator 网关的入口元数据拦截替换注解器
type Annotator func(context.Context, *http.Request, metadata.MD)

// WithMetadataAnnotator 配置 gRPC 网关的元数据拦截器
func WithMetadataAnnotator(annotator Annotator) HandlerOption {
	return func(gh *Handler) error {
		gh.annotators = append(gh.annotators, annotator)
		return nil
	}
}

// ErrorHandler 错误处理器
type ErrorHandler func(context.Context, *runtime.ServeMux, runtime.Marshaler, http.ResponseWriter, *http.Request, error) error

// WithErrorHandler 新增错误处理器
func WithErrorHandler(handler ErrorHandler) HandlerOption {
	return func(gh *Handler) error {
		gh.errHandlers = append(gh.errHandlers, handler)
		return nil
	}
}

// RoutingErrorHandler 路由错误处理器, 如果路由错误处理器返回 true 则终止后续其他的错误处理器
type RoutingErrorHandler func(context.Context, *runtime.ServeMux, runtime.Marshaler, http.ResponseWriter, *http.Request, int) bool

// WithRoutingErrorHandler 新增路由错误处理器
func WithRoutingErrorHandler(handler RoutingErrorHandler) HandlerOption {
	return func(gh *Handler) error {
		gh.routingErrorHandler = append(gh.routingErrorHandler, handler)
		return nil
	}
}

// WithRuntimeMuxOption 为网关配置配置一些额外属性
func WithRuntimeMuxOption(opts ...runtime.ServeMuxOption) HandlerOption {
	return func(gh *Handler) error {
		gh.muxOptions = append(gh.muxOptions, opts...)
		return nil
	}
}

// WithHijackRoutes 为网关配置路由, 在请求进入网关之后会优先匹配这些路由
func WithHijackRoutes(f func(r *mux.Router) error) HandlerOption {
	return func(gh *Handler) error {
		gh.muxRoutes = append(gh.muxRoutes, f)
		return nil
	}
}

// New 创建网关实例对象
func New(options ...HandlerOption) (*Handler, error) {
	ggs := &Handler{
		endpoint: "localhost",
		headers:  make(map[string]bool),
	}

	for _, option := range options {
		if err := option(ggs); err != nil {
			return nil, err
		}
	}

	muxOpts := []runtime.ServeMuxOption{
		runtime.WithErrorHandler(ggs.withErrorHandler),
		runtime.WithMetadata(ggs.withMetadataAnnotator),
		runtime.WithRoutingErrorHandler(ggs.withRoutingErrorHandler),
		runtime.WithOutgoingHeaderMatcher(ggs.withOutgoingHeaderMatcher),
		runtime.WithMarshalerOption(multipart.MIME, &multipart.Marshaler{}),
		runtime.WithForwardResponseOption(ggs.withForwardResponseOption),
	}
	muxOpts = append(muxOpts, ggs.muxOptions...)
	ggs.mux = runtime.NewServeMux(muxOpts...)

	return ggs, nil
}
