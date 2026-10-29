package conf_type

import "time"

// Pgsql Pgsql
type Pgsql struct {
	Startup bool   `yaml:"_startup"`
	URL     string `yaml:"url"`

	HiddenSQL bool `yaml:"hidden_sql"`

	MaxIdleConnection int           `yaml:"max_idle_connection"`
	MaxOpenConnection int           `yaml:"max_open_connection"`
	MaxIdleTime       time.Duration `yaml:"max_idle_time"`
}

// Pgsqls Pgsqls
type Pgsqls map[string]Pgsql
