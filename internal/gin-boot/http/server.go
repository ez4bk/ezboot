package http

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ez4bk/ezboot/bizerr"
)

// Driver ...
type Driver = gin.Engine
type HandlerFunc = gin.HandlerFunc
type Handler = func(server *Driver)
type IServer interface {
	BindHandler(handlers ...func(server *gin.Engine)) *IServer
	Run()
}

// Resp is the http response struct
type Resp struct {
	Success bool   `json:"success"`
	Code    uint32 `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// SwaggerRespMeta SwaggerRespMeta
type SwaggerRespMeta struct {
	// example: true
	Success bool `json:"success"`
	// example: 0
	Code uint32 `json:"code"`
	// example: ""
	Message string `json:"message"`
}

// SwaggerErrorMeta SwaggerErrorMeta
type SwaggerErrorMeta struct {
	// example: false
	Success bool `json:"success"`
	// example: nil
	Data interface{} `json:"data"`
}

// Ok Ok
func Ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Resp{
		Success: true,
		Data:    data,
	})
}

// Err Err
func Err(c *gin.Context, code codes.Code, a ...interface{}) {
	c.JSON(http.StatusOK, Resp{
		Success: false,
		Code:    uint32(code),
		Message: fmt.Sprint(a...),
	})
}

// Errf Errf
func Errf(c *gin.Context, code codes.Code, format string, a ...interface{}) {
	c.JSON(http.StatusOK, Resp{
		Success: false,
		Code:    uint32(code),
		Message: fmt.Sprintf(format, a...),
	})
}

// ErrFromGrpc ErrFromGrpc
func ErrFromGrpc(c *gin.Context, err error) {
	ErrFromGrpcWithStatus(c, err, http.StatusOK)
}

// ErrFromGrpcWithStatus ErrFromGrpcWithStatus
func ErrFromGrpcWithStatus(c *gin.Context, err error, httpStatus int) {
	if bizErr, ok := bizerr.FromError(err); ok {
		statusCode := httpStatus
		if bizErr.HttpCode() != 0 {
			statusCode = int(bizErr.HttpCode())
		}
		c.JSON(statusCode, Resp{
			Success: false,
			Code:    bizErr.Code(),
			Message: bizErr.Error(),
		})
		return
	}

	if s, ok := status.FromError(err); ok {
		c.JSON(httpStatus, Resp{
			Success: false,
			Code:    uint32(s.Code()),
			Message: s.Message(),
		})
		return
	}

	c.JSON(httpStatus, Resp{
		Success: false,
		Code:    uint32(codes.Unknown),
		Message: err.Error(),
	})
}

type unmarshal struct {
	Success bool   `json:"success"`
	Code    uint32 `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// GRPCErrFromHTTP GRPCErrFromHTTP
func GRPCErrFromHTTP(c *http.Response, unknownCode codes.Code) error {
	bs, err := ioutil.ReadAll(c.Body)
	if err != nil {
		return status.Error(unknownCode, err.Error())
	}
	payload := &unmarshal{}
	err = json.Unmarshal(bs, payload)
	if err != nil {
		return status.Errorf(unknownCode, "%v [%v]", err.Error(), string(bs))
	}
	httpStatus := c.StatusCode
	if httpStatus == http.StatusOK {
		httpStatus = http.StatusBadRequest
	}
	return bizerr.New(payload.Code, uint32(httpStatus), payload.Message)
}
