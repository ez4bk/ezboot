package exorm

import (
	"errors"
	"fmt"
)

// TokenPager 表示 AIP-158 风格的 page_size + page_token 分页参数
type TokenPager interface {
	// GetPageSize 分页大小, 需要大于 0
	GetPageSize() int64

	// GetPageToken 上一页返回的 next_page_token, 为空表示第一页
	GetPageToken() string
}

// TokenLimitWith 使用自定义 Token 配置游标分页.
//
// 调用方需要按当前排序方式传入对应的 decoder; token 与排序不匹配时
// 返回 ErrInvalidPageToken, 客户端应丢弃 token 并从第一页重新加载.
func (s *Session) TokenLimitWith(p TokenPager, decode TokenDecoder) (*Session, error) {
	if p == nil || p.GetPageSize() <= 0 {
		return s, nil
	}

	if pageToken := p.GetPageToken(); pageToken != "" {
		if err := s.applyPageToken(pageToken, decode); err != nil {
			return nil, err
		}
	}

	s.db.Limit(int(p.GetPageSize()))
	return s, nil
}

// applyPageToken 解析 page_token 并追加游标条件
func (s *Session) applyPageToken(pageToken string, decode TokenDecoder) error {
	if len(pageToken) > maxPageTokenLen || decode == nil {
		return ErrInvalidPageToken
	}

	token, err := decode(pageToken)
	if err != nil {
		if errors.Is(err, ErrInvalidPageToken) {
			return err
		}
		return fmt.Errorf("%w: %v", ErrInvalidPageToken, err)
	}
	if token == nil {
		return ErrInvalidPageToken
	}

	query, args := token.Filter()
	if query == "" {
		return ErrInvalidPageToken
	}

	s.db.Where(query, args...)
	return nil
}

// NextPageToken 根据本页最后一行构造的游标生成下一页 token.
//
// 本次返回条数不足 pageSize 时表示没有下一页, 返回空字符串.
// 使用 page_size+1 预取时, 应先裁剪到 page_size 再调用.
func NextPageToken(last Token, returned int, pageSize int64) string {
	if last == nil || pageSize <= 0 || returned <= 0 || int64(returned) < pageSize {
		return ""
	}
	return last.Encode()
}
