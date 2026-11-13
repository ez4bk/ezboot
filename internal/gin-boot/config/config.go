package config

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"github.com/ez4bk/ezboot/internal/gin-boot/conf_type"
	"github.com/ez4bk/ezboot/internal/gin-boot/env"
	"github.com/ez4bk/ezboot/internal/gin-boot/util"
)

const (
	RewriteFileName = "config_rewrite.yml"
	// ConfigSecretEnv is the environment variable holding the AES-GCM key for
	// encrypted MySQL DSN values.
	ConfigSecretEnv = "EZ_CONFIG_SECRET"
)

// Config Config
type Config struct {
	App struct {
		Name                 string
		Ver                  string
		Host                 string
		Port                 int
		LoadRemoteConfig     bool   `yaml:"load_remote_config"`
		NetworkInterfaceName string `yaml:"network_if_name"`
		PprofHost            string `yaml:"pprof_host"`
		PprofPort            int    `yaml:"pprof_port"`
		DBMigration          struct {
			AutoMigrate bool `yaml:"auto_migrate"`
		} `yaml:"db_migration"`

		Lifecycle conf_type.Lifecycle `yaml:"lifecycle"`

		Middleware struct {
			Logger  conf_type.Logger
			HTTP    conf_type.HTTP
			GRPC    conf_type.GRPC
			Monitor conf_type.Monitor
			Cache   conf_type.Caches
			APM     conf_type.APM
			Tracing conf_type.Tracing

			Mysql         conf_type.Mysqls
			Pgsql         conf_type.Pgsqls
			Sqlite        conf_type.Sqlites
			Redis         conf_type.Redises
			Etcd          conf_type.Etcd
			RabbitMQ      conf_type.RabbitMQ
			Kafka         conf_type.Kafka
			Elasticsearch conf_type.Elasticsearches
			Temporal      conf_type.Temporal
		}
	}
}

const (
	ConfigDir = "config"
)

var (
	Conf *Config
)

// InitConfig InitConfig
func InitConfig(filepath string) {
	f, err := os.Open(filepath)
	util.ChkErr(err)
	defer f.Close()

	Conf = &Config{}
	d := yaml.NewDecoder(f)

	if env.Env.LogLevel < env.LevelWarn {
		d.SetStrict(true)
	}

	util.ChkErr(d.Decode(Conf))

	// Environment variables take precedence over the values decoded above.
	// Credentials belong in environment variables so deployments can inject
	// them from Kubernetes Secrets instead of committing them to config files.
	applyEnvOverrides(Conf)

	// Decrypt mysql username and password if "Mysql.Encrypt" is true.
	for k, v := range Conf.App.Middleware.Mysql {
		if !v.Encrypt {
			continue
		}

		secret := os.Getenv(ConfigSecretEnv)
		if secret == "" {
			util.ChkErr(fmt.Errorf("mysql %q is encrypted but %s is not set", k, ConfigSecretEnv))
		}

		decryptDsn, err := decryptMysqlDsn(v.Dsn, path.Base(filepath), secret)
		util.ChkErr(err)
		v.Dsn = decryptDsn
		Conf.App.Middleware.Mysql[k] = v
	}
}

// ReadConfig ReadConfig for custom
func ReadConfig(configName string, configPath string, configType string) error {
	viper.SetConfigName(configName)
	viper.AddConfigPath(configPath)
	viper.SetConfigType(configType)
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}
	return nil
}

// Decrypt mysql username and password
func decryptMysqlDsn(dsn, configName, secret string) (string, error) {
	atSep, colonSep := "@", ":"
	var newDsnBuilder strings.Builder
	dbAccount := strings.Split(dsn, atSep)[0]
	dsnSuffix := strings.Replace(dsn, dbAccount, "", 1)
	errorMsg := "mysql username %v password is empty in %v config file, the invalid dsn is [%v]"

	if dbAccount == "" {
		return "", fmt.Errorf(errorMsg, "and", configName, dsn)
	}

	dbAccounts := strings.Split(dbAccount, colonSep)
	if len(dbAccounts) != 2 {
		return "", fmt.Errorf(errorMsg, "or", configName, dsn)
	}

	// Decrypt username and password
	for i, v := range dbAccounts {
		cipherText := strings.TrimSpace(v)
		if len(cipherText) == 0 {
			return "", fmt.Errorf(errorMsg, "or", configName, dsn)
		}

		plainText, err := util.Decrypt(v, secret)
		if err != nil {
			return "", fmt.Errorf(errorMsg, "or", configName, dsn)
		}
		newDsnBuilder.WriteString(plainText)
		if i == 0 {
			newDsnBuilder.WriteString(colonSep)
		}
	}

	newDsnBuilder.WriteString(dsnSuffix)
	return newDsnBuilder.String(), nil
}
