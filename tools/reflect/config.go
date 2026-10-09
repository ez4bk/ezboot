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
	DAOPolicy   *DAOPolicy               `json:"dao_policy" yaml:"dao_policy"`
	CachePolicy *CachePolicy             `json:"cache_policy" yaml:"cache_policy"`
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
	Required     bool   `json:"optional" yaml:"optional"`
	Likely       bool   `json:"likely" yaml:"likely"`
	Nullable     bool   `json:"nullable" yaml:"nullable"`
	OptionalSkip any    `json:"optional_skip" yaml:"optional_skip"`
	OptionalEnum string `json:"optional_enum" yaml:"optional_enum"`
}

// CachePolicy 针对缓存的生成策略
type CachePolicy struct {
	Prefix  string        `json:"prefix" yaml:"prefix"`
	Expired time.Duration `json:"expired" yaml:"expired"`
}
