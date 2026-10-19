package exorm

import (
	"encoding/base64"
	"errors"
	"strconv"
)

// ErrInvalidPageToken 表示 page_token 无法解析
var ErrInvalidPageToken = errors.New("exorm: invalid page token")

// TokenPager 表示 AIP-158 风格的 page_size + page_token 分页参数
type TokenPager interface {
	// GetPageSize 分页大小, 需要大于 0
	GetPageSize() int64

	// GetPageToken 上一页返回的 next_page_token, 为空表示第一页
	GetPageToken() string
}

// LimitToken 为查询配置 AIP-158 的游标分页
//
// token 中保存的是上一页最后一条记录的 cursor 值, 这里会追加
// column < cursor 条件, 并且只设置 limit, 不产生 offset.
// 该条件对应 ORDER BY column DESC; 如果改成 ASC, 需要相应改为 column > cursor.
// column 是游标列(通常是主键), 必须是可信的代码常量, 不能来自请求参数.
func (s *Session) LimitToken(p TokenPager, column string) *Session {
	if p == nil || p.GetPageSize() <= 0 {
		return s
	}

	if token := p.GetPageToken(); token != "" {
		cursor := DecodePageToken(token)
		s.db.Where(column+" < ?", cursor)
	}

	s.db.Limit(int(p.GetPageSize()))
	return s
}

// EncodePageToken 将游标编码为 page_token
// 游标通常是上一页最后一条记录的主键
func EncodePageToken(cursor int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(cursor, 10)))
}

// DecodePageToken 解析 page_token
func DecodePageToken(token string) (cursor int64) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0
	}

	cursor, err = strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return 0
	}
	return cursor
}

// NextPageToken 根据游标、本次返回条数和分页大小生成下一页 token
// 本次返回条数不足 pageSize 时表示没有下一页
func NextPageToken(cursor int64, returned int, pageSize int64) string {
	if pageSize <= 0 || returned <= 0 || int64(returned) < pageSize {
		return ""
	}
	return EncodePageToken(cursor)
}
