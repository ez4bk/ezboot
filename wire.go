package ezboot

import "github.com/google/wire"

// ProviderSet 提供基础的依赖配置
var ProviderSet = wire.NewSet(
	ProvideConfig,
	ProvideLogger,
)
