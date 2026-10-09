package reflect

import (
	"github.com/dave/jennifer/jen"
	"xorm.io/xorm/schemas"
)

// FieldType 表示一个结构体字段的类型信息
type FieldType struct {
	Value      any
	Identifier *QualifiedIdentifier
}

// Field 定义了数据库中的字段和当前结构体字段的属性
type Field struct {
	Column *schemas.Column
	Name   string
	Type   *FieldType

	stmt *jen.Statement
}
