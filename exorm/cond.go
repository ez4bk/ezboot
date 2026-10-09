package exorm

import "xorm.io/xorm"

// Session 增强版的 xorm 构建工具
type Session struct {
	db *xorm.Session
}

// Cond 条件语法糖, 只有在满足条件的情况下才增加后面的 query & args
func (s *Session) Cond(cond bool, query any, args ...any) *Session {
	if cond {
		s.db.Where(query, args...)
	}
	return s
}

// Nullable 条件语法糖, 只有在满足条件的情况下才增加后面的 query & f
func (s *Session) Nullable(cond bool, query any, f func() any) *Session {
	if cond {
		s.db.Where(query, f())
	}
	return s
}

// Raw 返回原始的 xorm 对象
func (s *Session) Raw() *xorm.Session {
	return s.db
}

// New 创建一个增强版的 xorm
func New(db *xorm.Session) *Session {
	return &Session{db: db}
}
