package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ez4bk/ezboot/internal/gin-boot/util"
)

func writeConfigFile(t *testing.T, raw string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "local.yml")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

func restoreConf(t *testing.T) {
	t.Helper()

	original := Conf
	t.Cleanup(func() { Conf = original })
}

const credentialConfig = `
app:
  name: demo
  middleware:
    mysql:
      default:
        _startup: true
        dsn: "file-user:file-pass@tcp(127.0.0.1:3306)/file"
        hidden_sql: true
        max_open_connection: 25
    pgsql:
      default:
        _startup: true
        url: "postgres://file:file@127.0.0.1:5432/file"
        hidden_sql: true
        max_idle_connection: 7
    redis:
      default:
        _startup: true
        addr: "127.0.0.1:6379"
        password: "file-redis-password"
        db: 3
    elasticsearch:
      default:
        _startup: true
        addresses:
          - "http://127.0.0.1:9200"
        username: "file-es-user"
        password: "file-es-password"
    rabbitmq:
      _startup: true
      url: "amqp://file:file@127.0.0.1:5672/"
`

func TestInitConfigEnvOverridesCredentials(t *testing.T) {
	restoreConf(t)

	t.Setenv("EZ_MYSQL_DEFAULT_DSN", "env-user:env-pass@tcp(127.0.0.1:3306)/env")
	t.Setenv("EZ_PGSQL_DEFAULT_URL", "postgres://env:env@127.0.0.1:5432/env")
	t.Setenv("EZ_REDIS_DEFAULT_PASSWORD", "env-redis-password")
	t.Setenv("EZ_ES_DEFAULT_PASSWORD", "env-es-password")
	t.Setenv("EZ_RABBITMQ_URL", "amqp://env:env@127.0.0.1:5672/")

	InitConfig(writeConfigFile(t, credentialConfig))

	mysql := Conf.App.Middleware.Mysql["default"]
	if mysql.Dsn != "env-user:env-pass@tcp(127.0.0.1:3306)/env" {
		t.Fatalf("mysql dsn = %q", mysql.Dsn)
	}
	if !mysql.HiddenSQL || mysql.MaxOpenConnection != 25 {
		t.Fatalf("mysql non-secret options were dropped: %+v", mysql)
	}

	pgsql := Conf.App.Middleware.Pgsql["default"]
	if pgsql.URL != "postgres://env:env@127.0.0.1:5432/env" {
		t.Fatalf("pgsql url = %q", pgsql.URL)
	}
	if !pgsql.HiddenSQL || pgsql.MaxIdleConnection != 7 {
		t.Fatalf("pgsql non-secret options were dropped: %+v", pgsql)
	}

	redis := Conf.App.Middleware.Redis["default"]
	if redis.Password != "env-redis-password" {
		t.Fatalf("redis password = %q", redis.Password)
	}
	if redis.Addr != "127.0.0.1:6379" || redis.DB != 3 {
		t.Fatalf("redis non-secret options were dropped: %+v", redis)
	}

	es := Conf.App.Middleware.Elasticsearch["default"]
	if es.Username != "file-es-user" || es.Password != "env-es-password" {
		t.Fatalf("elasticsearch credentials = %q / %q", es.Username, es.Password)
	}
	if !es.Startup || len(es.Addresses) != 1 || es.Addresses[0] != "http://127.0.0.1:9200" {
		t.Fatalf("elasticsearch non-secret options were dropped: %+v", es)
	}

	if Conf.App.Middleware.RabbitMQ.Url != "amqp://env:env@127.0.0.1:5672/" {
		t.Fatalf("rabbitmq url = %q", Conf.App.Middleware.RabbitMQ.Url)
	}
	if !Conf.App.Middleware.RabbitMQ.Startup {
		t.Fatal("rabbitmq startup was dropped")
	}
}

func TestInitConfigIgnoresEmptyEnvOverride(t *testing.T) {
	restoreConf(t)

	t.Setenv("EZ_MYSQL_DEFAULT_DSN", "")
	t.Setenv("EZ_REDIS_DEFAULT_PASSWORD", "")
	t.Setenv("EZ_RABBITMQ_URL", "")

	InitConfig(writeConfigFile(t, credentialConfig))

	if got := Conf.App.Middleware.Mysql["default"].Dsn; got != "file-user:file-pass@tcp(127.0.0.1:3306)/file" {
		t.Fatalf("mysql dsn = %q", got)
	}
	if got := Conf.App.Middleware.Redis["default"].Password; got != "file-redis-password" {
		t.Fatalf("redis password = %q", got)
	}
	if got := Conf.App.Middleware.RabbitMQ.Url; got != "amqp://file:file@127.0.0.1:5672/" {
		t.Fatalf("rabbitmq url = %q", got)
	}
}

func TestInitConfigDecryptsMysqlAndKeepsOptions(t *testing.T) {
	restoreConf(t)

	const secret = "config-secret"
	user, err := util.Encrypt("user", secret)
	if err != nil {
		t.Fatal(err)
	}
	password, err := util.Encrypt("password", secret)
	if err != nil {
		t.Fatal(err)
	}

	raw := fmt.Sprintf(`
app:
  middleware:
    mysql:
      default:
        _startup: true
        encrypt: true
        dsn: '%s:%s@tcp(127.0.0.1:3306)/demo'
        hidden_sql: true
        max_open_connection: 25
`, user, password)

	t.Setenv(ConfigSecretEnv, secret)
	t.Setenv("EZ_MYSQL_DEFAULT_DSN", "")

	InitConfig(writeConfigFile(t, raw))

	mysql := Conf.App.Middleware.Mysql["default"]
	const want = "user:password@tcp(127.0.0.1:3306)/demo"
	if mysql.Dsn != want {
		t.Fatalf("mysql dsn = %q, want %q", mysql.Dsn, want)
	}
	if !mysql.HiddenSQL || mysql.MaxOpenConnection != 25 || !mysql.Encrypt {
		t.Fatalf("mysql options were dropped after decryption: %+v", mysql)
	}
}

func TestInitConfigElasticsearchIgnoresUsernameOverride(t *testing.T) {
	restoreConf(t)

	t.Setenv("EZ_ES_DEFAULT_USERNAME", "env-es-user")
	t.Setenv("EZ_ES_DEFAULT_PASSWORD", "")

	InitConfig(writeConfigFile(t, credentialConfig))

	es := Conf.App.Middleware.Elasticsearch["default"]
	if es.Username != "file-es-user" {
		t.Fatalf("elasticsearch username = %q, want the YAML value", es.Username)
	}
	if es.Password != "file-es-password" {
		t.Fatalf("elasticsearch password = %q, want the YAML value", es.Password)
	}
	if !es.Startup || len(es.Addresses) != 1 {
		t.Fatalf("elasticsearch non-secret options were dropped: %+v", es)
	}
}

func TestInitConfigEnvOverridesNamedInstance(t *testing.T) {
	restoreConf(t)

	const raw = `
app:
  middleware:
    redis:
      report:
        addr: "127.0.0.1:6380"
        password: "file-password"
`
	t.Setenv("EZ_REDIS_REPORT_PASSWORD", "env-password")

	InitConfig(writeConfigFile(t, raw))

	if got := Conf.App.Middleware.Redis["report"].Password; got != "env-password" {
		t.Fatalf("redis report password = %q", got)
	}
	if got := Conf.App.Middleware.Redis["report"].Addr; got != "127.0.0.1:6380" {
		t.Fatalf("redis report addr = %q", got)
	}
}
