package reflect

import (
	_errors "errors"
	"io"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

var (
	// ErrTableNotFound 表示找不到相对应的表
	ErrTableNotFound = _errors.New("reflect: table not found")
)

// Reflection 是一个用于反射数据库表结构的工具
type Reflection struct {
	engine *xorm.Engine

	tables map[string]*schemas.Table
	policy *Policy
}

// Tables 获取所有的表名
func (r *Reflection) Tables() []string {
	var tables []string
	for table := range r.tables {
		tables = append(tables, table)
	}

	sort.Strings(tables)
	return tables
}

// Schema 获取某一个表的配置
func (r *Reflection) Schema(table string) *schemas.Table {
	return r.tables[table]
}

// Render 将指定的表中的数据反射到Writer中
func (r *Reflection) Render(table string, w io.Writer, options ...RenderOption) error {
	found, ok := r.tables[table]
	if !ok {
		return ErrTableNotFound
	}

	rr, err := newRender(found, r.mergeRenderOptions(table, options)...)
	if err != nil {
		return err
	}

	return rr.Render(w)
}

// mergeRenderOptions 合并表格的生成配置项
func (r *Reflection) mergeRenderOptions(table string, options []RenderOption) []RenderOption {
	var merged []RenderOption
	if r.policy != nil {
		if policy, ok := r.policy.Tables[table]; ok {
			if len(policy.Alias) != 0 {
				merged = append(merged, WithStructNameMapper(StringMapperFunc(func(string) string {
					return policy.Alias
				})))
			}

			if len(policy.Package) != 0 {
				merged = append(merged, WithPkgName(policy.Package))
			}

			if policy.DAOPolicy != nil {
				merged = append(merged, WithDAORenderOptions(r.mergeDAORenderOptions(policy.DAOPolicy)...))
			}

			if policy.CachePolicy != nil {
				merged = append(merged, WithDAORenderOptions(WithDAOCache(policy.CachePolicy.Prefix, policy.CachePolicy.Expired)))
			}

			for name, column := range policy.Columns {
				if len(column.Alias) != 0 {
					copyName := name
					copyAlias := column.Alias
					merged = append(merged, WithFiledAliasMapper(StringMapperFunc(func(column string) string {
						if column == copyName {
							return copyAlias
						}
						return ""
					})))
				}
				if column.TypeValue != nil {
					merged = append(merged, WithFieldTypeMapperByName(name, column.TypeValue))
				}
			}
		}
	}

	return append(merged, options...)
}

// mergeDAORenderOptions 合并生成数据访问层配置
func (r *Reflection) mergeDAORenderOptions(policy *DAOPolicy) []DAORenderOption {
	var options []DAORenderOption
	if len(policy.Package) != 0 {
		options = append(options, WithDAOPkgName(policy.Package))
	}

	for _, field := range sortMapKey(policy.Constraint) {
		constraint := policy.Constraint[field]

		if constraint.Likely {
			options = append(options, WithDAOListerConstraintLikely(field, !constraint.Required))
		} else {
			options = append(options, WithDAOListerConstraint(field,
				WithConstraintRequired(constraint.Required),
				WithConstraintOptional(constraint.OptionalSkip != nil),
				WithConstraintAsEnumValue(constraint.OptionalEnum),
				WithConstraintNullable(constraint.Nullable),
				WithConstraintSkipCond(constraint.OptionalSkip),
			))
		}
	}

	return options
}

var (
	// StopVisit 表示停止遍历表
	StopVisit = _errors.New("visit: stop")
)

// Visit 遍历所有反射的表的名字, 如果返回 StopVisit 则停止遍历并返回 nil
func (r *Reflection) Visit(visit func(table string) error) error {
	for _, table := range r.Tables() {
		if err := visit(table); err != nil {
			if _errors.Is(err, StopVisit) {
				return nil
			}
			return err
		}
	}
	return nil
}

// Close 清理反射工具的资源
func (r *Reflection) Close() error {
	return r.engine.Close()
}

// init 初始化并导出数据库的表结构信息
func (r *Reflection) init() error {
	// 添加 POINT 类型支持
	if _, ok := schemas.SqlTypes["POINT"]; !ok {
		schemas.SqlTypes["POINT"] = schemas.BLOB_TYPE
	}

	tables, err := r.engine.DBMetas()
	if err != nil {
		return errors.Wrap(err, "failed to retrieves the table schemas")
	}

	r.tables = make(map[string]*schemas.Table)
	for _, table := range tables {
		r.tables[table.Name] = table
	}

	return nil
}

// Option 表示创建反射工具的额外配置项目
type Option func(r *Reflection) error

// WithPolicy 配置表生成配置
func WithPolicy(policy *Policy) Option {
	return func(r *Reflection) error {
		r.policy = policy
		return nil
	}
}

// New 创建一个数据库反射实例对象
func New(dsn string, options ...Option) (*Reflection, error) {
	driver, rest, err := splitScheme(dsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse the dsn for database")
	}

	engine, err := xorm.NewEngine(driver, rest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create database engine")
	}

	return NewWithEngine(engine, options...)
}

// NewWithEngine 根据传入的具体ORM引擎创建反射对象
func NewWithEngine(engine *xorm.Engine, options ...Option) (*Reflection, error) {
	r := &Reflection{engine: engine}
	if err := r.init(); err != nil {
		return nil, err
	}

	for _, option := range options {
		if err := option(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// splitScheme 将传入的DSN信息按照分隔符进行分开
func splitScheme(dsn string) (string, string, error) {
	segments := strings.SplitN(dsn, "://", 2)
	if len(segments) != 2 {
		return "", "", errors.New("the DSN mismatched the form driver://user:pass@host/database")
	}
	return segments[0], segments[1], nil
}

// sortMapKey 排序字典的键值对
func sortMapKey[V any](m map[string]V) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}
