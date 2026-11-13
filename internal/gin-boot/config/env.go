package config

import (
	"fmt"
	"os"
	"strings"
)

// applyEnvOverrides replaces password-bearing values with values from
// environment variables. This keeps secrets out of the YAML files and lets
// deployments inject them from Kubernetes Secrets.
//
// The variable name is EZ_<PLUGIN>_<NAME>_<FIELD>, where <PLUGIN> and
// <FIELD> are upper-cased and <NAME> is the key of the named instance:
//
//	EZ_MYSQL_DEFAULT_DSN
//	EZ_PGSQL_DEFAULT_URL
//	EZ_REDIS_DEFAULT_PASSWORD
//	EZ_ES_DEFAULT_PASSWORD
//	EZ_RABBITMQ_URL
//
// PostgreSQL and RabbitMQ are overridden through their URL because that is
// where their credentials live. Plugins configured as a single instance
// (RabbitMQ) omit <NAME>. Empty environment variables are ignored so the
// YAML value stays in effect.
func applyEnvOverrides(cfg *Config) {
	for name, c := range cfg.App.Middleware.Mysql {
		if dsn := os.Getenv(fmt.Sprintf("EZ_MYSQL_%v_DSN", strings.ToUpper(name))); dsn != "" {
			c.Dsn = dsn
			cfg.App.Middleware.Mysql[name] = c
		}
	}

	for name, c := range cfg.App.Middleware.Pgsql {
		if url := os.Getenv(fmt.Sprintf("EZ_PGSQL_%v_URL", strings.ToUpper(name))); url != "" {
			c.URL = url
			cfg.App.Middleware.Pgsql[name] = c
		}
	}

	for name, c := range cfg.App.Middleware.Redis {
		if password := os.Getenv(fmt.Sprintf("EZ_REDIS_%v_PASSWORD", strings.ToUpper(name))); password != "" {
			c.Password = password
			cfg.App.Middleware.Redis[name] = c
		}
	}

	for name, c := range cfg.App.Middleware.Elasticsearch {
		if password := os.Getenv(fmt.Sprintf("EZ_ES_%v_PASSWORD", strings.ToUpper(name))); password != "" {
			c.Password = password
			cfg.App.Middleware.Elasticsearch[name] = c
		}
	}

	if url := os.Getenv("EZ_RABBITMQ_URL"); url != "" {
		cfg.App.Middleware.RabbitMQ.Url = url
	}
}
