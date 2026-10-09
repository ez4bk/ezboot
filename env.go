package ezboot

import (
	"os"
	"strings"
)

func Env() string {
	if env := os.Getenv("EZ_MODE"); len(env) != 0 {
		return strings.ToLower(env)
	} else if env = os.Getenv("EZ_ENV"); len(env) != 0 {
		return strings.ToLower(env)
	}
	return "local"
}

func IsProd() bool {
	return Env() == "prod"
}

func IsLocal() bool {
	return Env() == "local"
}

func LoggerLevel() string {
	return strings.ToLower(os.Getenv("EZ_LOG_LEVEL"))
}
