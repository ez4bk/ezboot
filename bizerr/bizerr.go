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
	bizCode    uint32
	httpCode   uint32
	template   string
	underlying error

	rendered string
}

// HttpCode 返回当前错误的Http状态码
func (e *BizError) HttpCode() uint32 {
	return e.httpCode
}

// Code 返回当前错误的业务错误码
func (e *BizError) Code() uint32 {
	return e.bizCode
}

// Render 渲染其中的错误模板消息
func (e *BizError) Render(args ...interface{}) *BizError {
	ne := *e
	ne.rendered = fmt.Sprintf(e.template, args...)

	return &ne
}

// WithCause 携带额外的底层错误信息
func (e *BizError) WithCause(err error) *BizError {
	ne := *e
	ne.underlying = err

	return &ne
}

// Error 业务的错误消息, 如果未指定模板则使用模板模板
func (e *BizError) Error() string {
	if len(e.rendered) != 0 {
		return e.rendered
	}
	return fmt.Sprintf("Error %d, Http %d", e.bizCode, e.httpCode)
}

// GRPCStatus 将业务的错误转换为一个 gRPC 状态消息
func (e *BizError) GRPCStatus() *status.Status {
	errStatus := status.New(codes.Code(e.bizCode), e.Error())

	var details []protoadapt.MessageV1
	details = append(details, &pberr.WithHttpStatus{HttpCode: e.httpCode})
	if e.underlying != nil {
		details = append(details, &pberr.WithCauseError{CauseError: e.underlying.Error()})
	}

	errStatus, _ = errStatus.WithDetails(details...)
	return errStatus
}

// Is 返回该错误的错误码是否与当前错误的错误码相同
func (e *BizError) Is(err error) bool {
	var bizErr *BizError
	if errors.As(err, &bizErr) {
		return bizErr.bizCode == e.bizCode
	}

	if errStatus, ok := status.FromError(err); ok {
		return errStatus.Code() == codes.Code(e.bizCode)
	}

	return false
}

// New 创建一个自定义的业务错误
func New(bizCode, httpCode uint32, template ...string) *BizError {
	bizError := &BizError{bizCode: bizCode, httpCode: httpCode}
	if len(template) != 0 {
		bizError.template = template[0]
		if strings.Count(bizError.template, "%") == 0 {
			bizError.rendered = bizError.template
		}
	}
	if bizError.httpCode == 0 {
		bizError.httpCode = http.StatusBadRequest
	}

	return bizError
}

// FromError 从一个错误接口中获取一个业务错误
func FromError(err error) (*BizError, bool) {
	var bizErr BizError
	if errStatus, ok := status.FromError(err); ok {
		bizErr.bizCode = uint32(errStatus.Code())
		for _, detail := range errStatus.Details() {
			switch v := detail.(type) {
			case *pberr.WithHttpStatus:
				bizErr.httpCode = v.HttpCode
			}
		}
		bizErr.rendered = errStatus.Message()
	}
	return &bizErr, bizErr.httpCode != 0
}

// ErrorRange 是一个用来自动创建范围错误码的工具类
type ErrorRange struct {
	index uint32
}

// New 创建一个自定义的业务错误
func (r *ErrorRange) New(http uint32, template ...string) *BizError {
	return New(atomic.AddUint32(&r.index, 1), http, template...)
}

// NewRange 创建一个范围错误码工具类
func NewRange(start uint32) *ErrorRange {
	return &ErrorRange{index: start}
}
