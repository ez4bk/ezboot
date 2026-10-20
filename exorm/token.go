package exorm

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

// ErrInvalidPageToken 表示 page_token 无法解析或与当前排序不匹配
var ErrInvalidPageToken = errors.New("exorm: invalid page token")

// maxPageTokenLen page_token 的最大长度, 防止构造超长输入
const maxPageTokenLen = 4096

// Token 表示 keyset 分页的游标, 即当前在某个排序序列中的位置.
//
// Token 只承载"从哪继续", 不承载筛选条件和权限;
// 排序方式由 token 的 kind 与 decoder 绑定, 仅用于拒绝错配,
// 实际使用哪种排序仍由请求参数决定.
type Token interface {
	// Encode 将游标编码为对客户端不透明的 page_token
	Encode() string

	// Filter 返回"位于该游标之后"的 keyset 条件.
	//
	// 约束:
	//   - query 必须是代码内常量(只含 ? 占位符), 禁止拼接 token 中的字符串;
	//   - args 与 ? 一一对应, 且只能来自 token 解码后的值;
	//   - 条件只能使用当前查询表的列/表达式, 只能表达分页位置,
	//     不得包含可见性、权限或业务筛选(这些由调用方用 Cond/Where 叠加).
	Filter() (query string, args []any)
}

// TokenDecoder 把 page_token 解析成 Token.
//
// 每个排序方式绑定一个 decoder; token 与排序不匹配时返回 ErrInvalidPageToken.
// 只有非空 token 会被传入.
type TokenDecoder func(pageToken string) (Token, error)

// EncodeToken 将游标编码为 <kind>.<version>.<base64url(payload)>.
//
// kind 是排序方式的稳定标识(不能包含 "."), 例如 id_desc、posts.distance_asc;
// version 是 token 格式版本. 不同排序方式必须使用不同的 kind,
// 这样错配的 token 会在解码阶段被拒绝.
func EncodeToken(kind string, version uint8, payload []byte) string {
	return kind + "." + strconv.Itoa(int(version)) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
}

// DecodeToken 校验 kind/version 并返回 payload.
func DecodeToken(kind string, version uint8, pageToken string) ([]byte, error) {
	if len(pageToken) == 0 || len(pageToken) > maxPageTokenLen {
		return nil, ErrInvalidPageToken
	}

	parts := strings.SplitN(pageToken, ".", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidPageToken
	}
	if parts[0] != kind || parts[1] != strconv.Itoa(int(version)) {
		return nil, ErrInvalidPageToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidPageToken
	}
	return payload, nil
}
