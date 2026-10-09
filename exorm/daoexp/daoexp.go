package daoexp

import (
	"context"

	"xorm.io/xorm"

	"github.com/ez4bk/ezboot/elog"
	"github.com/ez4bk/ezboot/with"
)

// GenericDAO 通用的数据库操作以及访问范型
type GenericDAO[T PrimaryKey[K], K any] struct {
	log *ezboot.Logger
}

// Create 在数据库中创建一条记录
func (dao *GenericDAO[T, K]) Create(ctx context.Context, f func(*T) error) (K, error) {
	var model T
	err := with.DefaultSession(ctx, func(db *xorm.Session) error {
		if err := f(&model); err != nil {
			return err
		}

		_, err := db.Insert(&model)
		if err != nil {
			return elog.Warnw(dao.log, err, "failed to insert the model into database")
		}
		return nil
	})

	return model.PK(), err
}

// Exists 检查数据库记录是否存在
func (dao *GenericDAO[T, K]) Exists(ctx context.Context, id K) (exists bool, err error) {
	var zero T
	err = with.DefaultSession(ctx, func(db *xorm.Session) error {
		exists, err = db.
			ID(id).
			Where("deleted = ?", false).
			Exist(&zero)
		return elog.Warnw(dao.log, err, "failed to check model exists in database")
	})
	return
}

// Update 更新一条数据库记录
func (dao *GenericDAO[T, K]) Update(ctx context.Context, model T, cols ...string) error {
	return with.DefaultSession(ctx, func(db *xorm.Session) error {
		_, err := db.ID(model.PK()).Where("deleted = ?", false).MustCols(cols...).Update(model)
		return elog.Warnw(dao.log, err, "failed to update the model from database")
	})
}

// PrimaryKey 返回当前模型的主键
type PrimaryKey[T any] interface {
	PK() T
}
