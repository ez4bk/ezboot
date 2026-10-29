package infra

import "github.com/ez4bk/ezboot/internal/logging"

// loggerOrDefault 返回调用方传入的 logger，缺省使用框架默认 logger。
func loggerOrDefault(logger *logging.Logger) *logging.Logger {
	if logger != nil {
		return logger
	}
	return logging.Default()
}
