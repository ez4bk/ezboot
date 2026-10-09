package bizerr

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

var (
	// InvalidArgument 表示用户参数错误
	InvalidArgument = New(uint32(codes.InvalidArgument), http.StatusBadRequest, "invalid arguments [%v]")

	// ServerInternalError 表示服务器内部错误
	ServerInternalError = New(uint32(codes.Internal), http.StatusInternalServerError, "server internal error [%v]")

	// MissingArgument 表示缺少必要参数
	MissingArgument = New(uint32(codes.InvalidArgument), http.StatusBadRequest, "missing arguments [%v]")
)
