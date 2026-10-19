package reflect

import "time"

// Policy 通过文件或者直接传入对象的方式配置生成策略
type Policy struct {
	Tables map[string]*TablePolicy `json:"tables" yaml:"tables"`
}

// TablePolicy 针对表的生成策略
type TablePolicy struct {
	Alias       string                   `json:"alias" yaml:"alias"`
	Package     string                   `json:"package" yaml:"package"`
	Columns     map[string]*ColumnPolicy `json:"columns" yaml:"columns"`
	PagerPolicy *PagerPolicy             `json:"pager_policy" yaml:"pager_policy"`
	DAOPolicy   *DAOPolicy               `json:"dao_policy" yaml:"dao_policy"`
	CachePolicy *CachePolicy             `json:"cache_policy" yaml:"cache_policy"`
}

// PagerType 分页参数类型
type PagerType string

const (
	// PagerTypeIndex 使用 page_size + page_index 分页
	PagerTypeIndex PagerType = "index"

	// PagerTypeToken 使用 AIP-158 的 page_size + page_token 分页
	PagerTypeToken PagerType = "token"
)

// PagerPolicy 分页参数的生成策略
type PagerPolicy struct {
	// Type 分页类型, 支持 index 和 token, 默认为 index
	Type PagerType `json:"type" yaml:"type"`
}

// ColumnPolicy 针对字段的生成策略
type ColumnPolicy struct {
	Alias     string `json:"alias" yaml:"alias"`
	TypeValue any    `json:"type_value" yaml:"type_value"`
}

// DAOPolicy 针对数据库访问层函数的生成策略
type DAOPolicy struct {
	Package    string                        `json:"package" yaml:"package"`
	Constraint map[string]*DAOListConstraint `json:"constraint" yaml:"constraint"`
}

// DAOListConstraint 数据访问层列表函数的字段配置
type DAOListConstraint struct {
	// Likely 标记当前字段使用模糊查询, 该字段天然是可选条件
	Likely bool `json:"likely" yaml:"likely"`
	// Nullable 标记当前字段是可选指针, 条件随参数是否为 nil 决定
	Nullable bool `json:"nullable" yaml:"nullable"`
	// OptionalSkip 配置可选条件的跳过值, 配置后该字段即为可选
	OptionalSkip any `json:"optional_skip" yaml:"optional_skip"`
	// OptionalEnum 是枚举常量的全限定名(import路径.常量名), 取它的 String() 作为跳过值, 配置后该字段即为可选
	OptionalEnum string `json:"optional_enum" yaml:"optional_enum"`
}

// CachePolicy 针对缓存的生成策略
type CachePolicy struct {
	Prefix  string        `json:"prefix" yaml:"prefix"`
	Expired time.Duration `json:"expired" yaml:"expired"`
}
