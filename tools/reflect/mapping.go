package reflect

import (
	"reflect"
	"strings"
	"time"

	"github.com/dave/jennifer/jen"
	"github.com/pkg/errors"
	"xorm.io/xorm/schemas"
)

// TypeMapper 是一个用于字段类型映射的接口类型
type TypeMapper interface {
	// Mapping 根据给定的字段和表格返回所需要使用的类型
	Mapping(column *schemas.Column, table *schemas.Table) any
}

// TypeMapperFunc 是一个使用闭包函数实现的类型映射方法
type TypeMapperFunc func(column *schemas.Column, table *schemas.Table) any

// Mapping 根据给定的字段和表格调用函数返回所需要使用的类型
func (f TypeMapperFunc) Mapping(column *schemas.Column, table *schemas.Table) any {
	return f(column, table)
}

// QualifiedIdentifier 是一个全限定的符号标识符
type QualifiedIdentifier struct {
	Pointer    bool   `json:"pointer" yaml:"pointer"`
	Import     string `json:"import" yaml:"import"`
	Identifier string `json:"identifier" yaml:"identifier"`
}

// Pass 将其作为参数返回
func (qi *QualifiedIdentifier) Pass() (string, string) {
	return qi.Import, qi.Identifier
}

// AsCode 将其转换为jen.Code类型用于生成代码
func (qi *QualifiedIdentifier) AsCode() jen.Code {
	stmt := &jen.Statement{}

	if qi.Pointer {
		stmt.Op("*")
	}

	if len(qi.Import) == 0 {
		return stmt.Id(qi.Identifier)
	}

	return stmt.Qual(qi.Import, qi.Identifier)
}

func typeName(rt reflect.Type) string {
	if name := rt.Name(); name != "" {
		return name
	}
	return rt.String()
}

// AsQualifiedIdentifier 获取指定值的全限定符号名
func AsQualifiedIdentifier(v interface{}) (*QualifiedIdentifier, error) {
	if v == nil {
		return nil, errors.New("nil value does not contain any type")
	}

	if qi, ok := v.(*QualifiedIdentifier); ok {
		return qi, nil
	}

	rt := reflect.TypeOf(v)
	pointer := rt.Kind() == reflect.Pointer
	if pointer {
		rt = rt.Elem()
	}

	return &QualifiedIdentifier{
		Pointer:    pointer,
		Import:     rt.PkgPath(),
		Identifier: typeName(rt),
	}, nil
}

// MustAsQualifiedIdentifier 获取指定值的全限定符号名
func MustAsQualifiedIdentifier(v interface{}) *QualifiedIdentifier {
	identifier, err := AsQualifiedIdentifier(v)
	if err != nil {
		panic(err)
	}
	return identifier
}

// ParseQualifiedIdentifier 解析指定值的全限定符号名
func ParseQualifiedIdentifier(s string) (*QualifiedIdentifier, error) {
	dot := strings.LastIndex(s, ".")
	imp, idf := s[:dot], s[dot+1:]
	if len(imp) == 0 || len(idf) == 0 {
		return nil, errors.New("bad qualified identifier")
	}
	return &QualifiedIdentifier{Import: imp, Identifier: idf}, nil
}

// MustParseQualifiedIdentifier 解析指定值的全限定符号名
func MustParseQualifiedIdentifier(s string) *QualifiedIdentifier {
	identifier, err := ParseQualifiedIdentifier(s)
	if err != nil {
		panic(err)
	}
	return identifier
}

// DefaultTypeMapper 默认的字段类型映射
func DefaultTypeMapper() TypeMapper {
	return TypeMapperFunc(func(column *schemas.Column, table *schemas.Table) any {
		switch column.SQLType.Name {
		case schemas.Varchar, schemas.Text:
			return ""
		case schemas.BigInt:
			return int64(0)
		case schemas.Int, schemas.MediumInt:
			return int32(0)
		case schemas.SmallInt:
			return int16(0)
		case schemas.TinyInt:
			if column.SQLType.DefaultLength == 1 {
				return true
			}
			return int8(0)
		case schemas.UnsignedBigInt:
			return uint64(0)
		case schemas.UnsignedInt, schemas.UnsignedMediumInt:
			return uint32(0)
		case schemas.UnsignedSmallInt:
			return uint8(0)
		case schemas.Float, schemas.Double, schemas.Decimal:
			return float64(0)
		case schemas.Bool, schemas.Boolean:
			return true
		case schemas.Date, schemas.DateTime, schemas.TimeStamp:
			return time.Time{}
		}
		return nil
	})
}

// WithFieldTypeMapper 添加一个自定义的类型映射
func WithFieldTypeMapper(mapper TypeMapper) RenderOption {
	return func(r *render) error {
		r.fieldTypeMappers = append(r.fieldTypeMappers, mapper)
		return nil
	}
}

// WithFieldTypeMapperByName 根据字段名字进行类型映射
func WithFieldTypeMapperByName(name string, v any) RenderOption {
	return WithFieldTypeMapper(TypeMapperFunc(func(column *schemas.Column, table *schemas.Table) any {
		if column.Name == name {
			return v
		}
		return nil
	}))
}
