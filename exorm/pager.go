package exorm

// Pager 表示 page_size + page_index 风格的分页参数
type Pager interface {
	// GetPageSize 分页大小, 需要大于 0
	GetPageSize() int64

	// GetPageIndex 分页页码
	// 索引从 1 开始, <= 1 均视为第一页
	GetPageIndex() int64
}

// Limit 为查询配置 page_size + page_index 分页
func (s *Session) Limit(p Pager) *Session {
	if p != nil && p.GetPageSize() > 0 {
		s.db.Limit(int(p.GetPageSize()), int((p.GetPageIndex()-1)*p.GetPageSize()))
	}
	return s
}
