package conf_type

import "github.com/ez4bk/ezboot/internal/gin-boot/util/custom"

// Lifecycle 控制插件各阶段的预算。零值表示使用框架默认值
// （ready 10s / drain 10s / close 5s）。
//
// 运维需要保证 terminationGracePeriodSeconds 大于
// Σ(drain) + Σ(close)，并留出余量。
type Lifecycle struct {
	ReadyTimeout custom.YAMLDuration `yaml:"ready_timeout"`
	DrainTimeout custom.YAMLDuration `yaml:"drain_timeout"`
	CloseTimeout custom.YAMLDuration `yaml:"close_timeout"`
}
