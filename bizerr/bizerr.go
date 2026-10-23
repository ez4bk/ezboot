package bizerr

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"

	"github.com/ez4bk/ezboot/internal/pberr"
)

// BizError 表示一个业务上的逻辑错误
type BizError struct {
	bizCode  uint32
	grpcCode codes.Code
	httpCode uint32
	template string

	underlying error
	rendered   string
}

// HttpCode 返回当前错误的 HTTP 状态码。
func (e *BizError) HttpCode() uint32 {
	return e.httpCode
}

// Code 返回当前错误的业务错误码。
func (e *BizError) Code() uint32 {
	return e.bizCode
}

// GRPCCode 返回真正通过 gRPC 传输的标准状态码。
func (e *BizError) GRPCCode() codes.Code {
	return e.grpcCode
}

// Render 渲染其中的错误模板消息。
func (e *BizError) Render(args ...interface{}) *BizError {
	ne := *e
	ne.rendered = fmt.Sprintf(e.template, args...)

	return &ne
}

// WithCause 携带额外的底层错误信息。
func (e *BizError) WithCause(err error) *BizError {
	ne := *e
	ne.underlying = err

	return &ne
}

// WithGRPCCode 覆盖默认推导出的 gRPC 状态码。
func (e *BizError) WithGRPCCode(code codes.Code) *BizError {
	ne := *e
	if code != codes.OK {
		ne.grpcCode = code
	}

	return &ne
}

// Error 返回业务错误消息。
func (e *BizError) Error() string {
	if len(e.rendered) != 0 {
		return e.rendered
	}
	return fmt.Sprintf("Error %d, Http %d", e.bizCode, e.httpCode)
}

// Unwrap 让 errors.Is/errors.As 能继续检查底层错误。
func (e *BizError) Unwrap() error {
	return e.underlying
}

// GRPCStatus 将业务错误转换为 gRPC 状态消息。
func (e *BizError) GRPCStatus() *status.Status {
	errStatus := status.New(e.grpcCode, e.Error())

	details := []protoadapt.MessageV1{
		&pberr.WithBizCode{BizCode: e.bizCode},
		&pberr.WithHttpStatus{HttpCode: e.httpCode},
	}
	if e.underlying != nil {
		details = append(details, &pberr.WithCauseError{CauseError: e.underlying.Error()})
	}

	errStatus, _ = errStatus.WithDetails(details...)
	return errStatus
}

// Is 按业务码比较错误，而不是按 gRPC status code 比较。
func (e *BizError) Is(target error) bool {
	var bizErr *BizError
	if errors.As(target, &bizErr) {
		return bizErr.bizCode == e.bizCode
	}

	if errStatus, ok := status.FromError(target); ok {
		for _, detail := range errStatus.Details() {
			if v, ok := detail.(*pberr.WithBizCode); ok {
				return v.BizCode == e.bizCode
			}
		}
	}

	return false
}

// New 创建一个自定义的业务错误。
func New(bizCode, httpCode uint32, template ...string) *BizError {
	bizError := &BizError{
		bizCode:  bizCode,
		httpCode: httpCode,
		grpcCode: grpcCodeForHTTP(httpCode),
	}
	if len(template) != 0 {
		bizError.template = template[0]
		if strings.Count(bizError.template, "%") == 0 {
			bizError.rendered = bizError.template
		}
	}
	if bizError.httpCode == 0 {
		bizError.httpCode = http.StatusBadRequest
		bizError.grpcCode = codes.InvalidArgument
	}

	return bizError
}

// FromError 从 gRPC 状态和 detail 中还原业务错误。
func FromError(err error) (*BizError, bool) {
	if err == nil {
		return nil, false
	}

	errStatus, ok := status.FromError(err)
	if !ok {
		return nil, false
	}

	bizErr := &BizError{
		bizCode:  uint32(errStatus.Code()),
		grpcCode: errStatus.Code(),
		rendered: errStatus.Message(),
	}

	isBiz := false
	for _, detail := range errStatus.Details() {
		switch v := detail.(type) {
		case *pberr.WithBizCode:
			bizErr.bizCode = v.BizCode
			isBiz = true
		case *pberr.WithHttpStatus:
			bizErr.httpCode = v.HttpCode
			isBiz = true
		case *pberr.WithCauseError:
			bizErr.underlying = errors.New(v.CauseError)
		}
	}

	if !isBiz {
		return nil, false
	}

	return bizErr, true
}

// ErrorRange 是一个用来自动创建范围错误码的工具类。
type ErrorRange struct {
	index uint32
}

// New 创建一个自定义的业务错误。
func (r *ErrorRange) New(http uint32, template ...string) *BizError {
	return New(atomic.AddUint32(&r.index, 1), http, template...)
}

// NewRange 创建一个范围错误码工具类。
func NewRange(start uint32) *ErrorRange {
	return &ErrorRange{index: start}
}

// grpcCodeForHTTP 从 HTTP 状态码推导一个标准 gRPC 状态码。
func grpcCodeForHTTP(httpCode uint32) codes.Code {
	switch httpCode {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.Aborted
	case http.StatusPreconditionFailed:
		return codes.FailedPrecondition
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case http.StatusNotImplemented:
		return codes.Unimplemented
	case http.StatusServiceUnavailable, http.StatusBadGateway:
		return codes.Unavailable
	case http.StatusGatewayTimeout:
		return codes.DeadlineExceeded
	default:
		return codes.Internal
	}
}
