package exorm

// Pager 表示一个可执行分页的参数请求
type Pager interface {
	// GetPageSize 分页大小, 需要大于 0
	GetPageSize() int64

	// GetPageIndex 分页页码, 索引从 0 开始
	GetPageIndex() int64
}

// Limit 为查询配置分页
func (s *Session) Limit(p Pager) *Session {
	if p != nil {
		s.db.Limit(int(p.GetPageSize()), int((p.GetPageIndex()-1)*p.GetPageSize()))
	}
	return s
}

type fixedPager struct {
	n int64
}

func (f fixedPager) GetPageSize() int64  { return f.n }
func (f fixedPager) GetPageIndex() int64 { return 1 }

// FixedPager 固定的分页大小
func FixedPager(n ...int64) Pager {
	if len(n) == 0 {
		return fixedPager{n: 1000}
	}
	return fixedPager{n: n[0]}
}
