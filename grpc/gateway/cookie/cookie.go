package cookie

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ez4bk/ezboot/grpc/gateway"
)

const (
	// _CookieMetadataPrefix 元数据名字前缀
	_CookieMetadataPrefix = "X-Cookie-"
)

var (
	ErrCookieNotFound = errors.New("cookie: not found")
	ErrCookieInvalid  = errors.New("cookie: invalid format")
)

// Annotation 为后续的请求创建元数据
func Annotation() gateway.Annotator {
	return func(ctx context.Context, req *http.Request, md metadata.MD) {
		for _, cookie := range req.Cookies() {
			md.Set(_CookieMetadataPrefix+cookie.Name, cookie.String())
		}
	}
}

// Send 发送一个Cookie
func Send(ctx context.Context, cookie *http.Cookie) error {
	return grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", cookie.String()))
}

// Load 从请求对象中加载一个Cookie
func Load(ctx context.Context, name string) (*http.Cookie, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if raws := md.Get(_CookieMetadataPrefix + name); len(raws) != 0 {
			cookies, err := parseCookie(raws[0])
			if err != nil {
				return nil, err
			}
			return cookies[0], nil
		}
	}

	return nil, ErrCookieNotFound
}

// parseCookie 解析 Cookie 字符串
func parseCookie(raw string) ([]*http.Cookie, error) {
	dummy := &http.Request{
		Header: http.Header{
			"Cookie": []string{raw},
		},
	}

	cookies := dummy.Cookies()
	if len(cookies) == 0 {
		return nil, ErrCookieInvalid
	}
	return cookies, nil
}

type NewOption func(c *http.Cookie)

func WithPath(path string) NewOption {
	return func(c *http.Cookie) {
		c.Path = path
	}
}

func WithExpires(expires time.Time) NewOption {
	return func(c *http.Cookie) {
		c.Expires = expires
	}
}

func WithSecure(secure bool) NewOption {
	return func(c *http.Cookie) {
		c.Secure = secure
	}
}

func New(name, value string, options ...NewOption) *http.Cookie {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}

	for _, apply := range options {
		apply(c)
	}

	return c
}
