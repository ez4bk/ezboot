package boot

import (
	"github.com/ez4bk/ezboot/internal/gin-boot/infra"
)

var (
	// Infra holds the process-level infrastructure Set opened by boot.Default.
	// It exists for legacy call sites only; new code should receive explicit
	// dependencies instead.
	Infra *infra.Set
)
