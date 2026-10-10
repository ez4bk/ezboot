package env

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Env_empty(t *testing.T) {
	t.Setenv(EnvNameMode, "")
	InitEnv("")
	t.Log(Env)
	assert.Equal(t, ModeLocal, Env.Mode)
	assert.Equal(t, "local", Env.Type)
	assert.Equal(t, LevelInfo, Env.LogLevel)
}

func Test_Env_ok(t *testing.T) {
	t.Setenv(EnvNameMode, "")
	InitEnv("ok.env")
	t.Log(Env)
	assert.Equal(t, ModeTest, Env.Mode)
	assert.Equal(t, "test", Env.Type)
	assert.Equal(t, LevelWarn, Env.LogLevel)
}

func Test_Env_err(t *testing.T) {
	t.Setenv(EnvNameMode, "")
	assert.Panics(t, func() {
		InitEnv("err.env")
	})
}

func Test_Env_os(t *testing.T) {
	t.Setenv(EnvNameMode, "")
	os.Setenv(EnvNameMode, "prod")
	InitEnv("")
	t.Log(Env)
	assert.Equal(t, ModeProd, Env.Mode)
}
