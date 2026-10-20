package exorm

import (
	"encoding/binary"
	"fmt"
	"reflect"
)

const (
	defaultTokenKind    = "id_desc"
	defaultTokenVersion = uint8(1)

	defaultTokenPKInt64  = byte(1)
	defaultTokenPKUint64 = byte(2)
	defaultTokenPKString = byte(3)
)

// DefaultToken 是只包含主键的游标, 对应 ORDER BY column DESC.
// 主键支持 int、uint、string 及其命名类型.
type DefaultToken struct {
	column string // 可信的代码常量, 不来自请求
	pk     any    // int64 | uint64 | string
}

// NewDefaultToken 创建一个默认主键游标.
// column 必须与查询的 ORDER BY 列一致.
// pk 支持 int、uint、string 及其命名类型; 其他类型会 panic(属于表结构/代码错误).
func NewDefaultToken(column string, pk any) DefaultToken {
	return DefaultToken{column: column, pk: normalizeDefaultPK(pk)}
}

// normalizeDefaultPK 将命名类型归一化为 int64/uint64/string
func normalizeDefaultPK(pk any) any {
	v := reflect.ValueOf(pk)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.String:
		return v.String()
	default:
		panic(fmt.Sprintf("exorm: unsupported default token primary key type %T", pk))
	}
}

// Encode 编码为默认游标的 page_token
func (t DefaultToken) Encode() string {
	return EncodeToken(defaultTokenKind, defaultTokenVersion, encodeDefaultPK(t.pk))
}

// encodeDefaultPK 编码为 1 字节类型标记 + 值
func encodeDefaultPK(pk any) []byte {
	switch v := pk.(type) {
	case int64:
		payload := make([]byte, 9)
		payload[0] = defaultTokenPKInt64
		binary.BigEndian.PutUint64(payload[1:], uint64(v))
		return payload
	case uint64:
		payload := make([]byte, 9)
		payload[0] = defaultTokenPKUint64
		binary.BigEndian.PutUint64(payload[1:], v)
		return payload
	case string:
		payload := make([]byte, 1+len(v))
		payload[0] = defaultTokenPKString
		copy(payload[1:], v)
		return payload
	default:
		panic(fmt.Sprintf("exorm: unsupported default token primary key type %T", pk))
	}
}

// Filter 返回默认游标的 keyset 条件: column < pk
func (t DefaultToken) Filter() (string, []any) {
	return t.column + " < ?", []any{t.pk}
}

// DefaultTokenDecoder 返回绑定到某一列的默认游标解码器.
func DefaultTokenDecoder(column string) TokenDecoder {
	return func(pageToken string) (Token, error) {
		payload, err := DecodeToken(defaultTokenKind, defaultTokenVersion, pageToken)
		if err != nil || len(payload) == 0 {
			return nil, ErrInvalidPageToken
		}

		switch payload[0] {
		case defaultTokenPKInt64:
			if len(payload) != 9 {
				return nil, ErrInvalidPageToken
			}
			return DefaultToken{
				column: column,
				pk:     int64(binary.BigEndian.Uint64(payload[1:])),
			}, nil
		case defaultTokenPKUint64:
			if len(payload) != 9 {
				return nil, ErrInvalidPageToken
			}
			return DefaultToken{
				column: column,
				pk:     binary.BigEndian.Uint64(payload[1:]),
			}, nil
		case defaultTokenPKString:
			return DefaultToken{
				column: column,
				pk:     string(payload[1:]),
			}, nil
		default:
			return nil, ErrInvalidPageToken
		}
	}
}

// DefaultTokenLimit 使用默认的主键游标配置分页.
//
// 默认 token 只包含主键, 追加 column < cursor 条件, 对应 ORDER BY column DESC.
// column 必须是唯一键(通常是主键), 且是可信的代码常量, 不能来自请求参数.
// 非法 page_token 返回 ErrInvalidPageToken.
func (s *Session) DefaultTokenLimit(p TokenPager, column string) (*Session, error) {
	return s.TokenLimitWith(p, DefaultTokenDecoder(column))
}

// EncodePageToken 将游标编码为 page_token.
//
// Deprecated: use NewDefaultToken(column, cursor).Encode instead.
// 注意: 编码格式已改为带 kind/version 外壳, 旧 token 不再兼容.
func EncodePageToken(cursor int64) string {
	return NewDefaultToken("", cursor).Encode()
}

// DecodePageToken 解析默认游标的 page_token.
//
// Deprecated: use DefaultTokenDecoder(column) instead.
// 注意: 旧格式 token 会解析失败并返回 0.
func DecodePageToken(token string) (cursor int64) {
	payload, err := DecodeToken(defaultTokenKind, defaultTokenVersion, token)
	if err != nil || len(payload) != 9 || payload[0] != defaultTokenPKInt64 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(payload[1:]))
}
