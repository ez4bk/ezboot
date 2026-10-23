package ezboot

import "github.com/google/wire"

// ProviderSet 提供所有项目都会用到的基础依赖。
var ProviderSet = wire.NewSet(
	ProvideConfig,
	ProvideLogger,
)
