package boot

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"go.elastic.co/apm"
	"go.elastic.co/apm/transport"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/env"
	"github.com/ez4bk/ezboot/internal/gin-boot/file"
	"github.com/ez4bk/ezboot/internal/gin-boot/grpc_boot"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware"
	"github.com/ez4bk/ezboot/internal/gin-boot/migration"
	"github.com/ez4bk/ezboot/internal/gin-boot/util"
	_ "github.com/ez4bk/ezboot/internal/gin-boot/validation"
	"github.com/ez4bk/ezboot/internal/gin-boot/version"
	"github.com/ez4bk/ezboot/internal/logging"
)

var (
	logger       *logging.Logger
	Env          env.EnvType
	Config       config.Config
	configLoaded bool
	Version      string
)

var mut sync.Mutex

var configPath string

var customLogLevel string

func LoadFrameworkConfig() error {
	if configLoaded {
		return nil
	}

	// import docker secrets as env
	env.ImportDockerSecretAsEnv()

	// init env
	env.InitEnv(`.env`)
	Env = *env.Env
	modeStr := env.ModeName(env.Env.Mode)
	logMode := "mode: " + modeStr

	// init config
	if configPath == "" {
		configPath = config.ConfigDir
	}
	configFilePath := configPath + string(os.PathSeparator) + modeStr + ".yml"
	logConfig := "config file: " + configFilePath

	config.InitConfig(configFilePath)
	{
		rewriteFile := path.Join(configPath, config.RewriteFileName)
		if f, e := os.Open(rewriteFile); e == nil {
			defer f.Close()

			d := yaml.NewDecoder(f)
			util.ChkErr(d.Decode(config.Conf))
		}
	}

	err := config.ReadConfig(modeStr+"_custom", configPath, "yaml")
	if err != nil {
		// _custom file is optional, ignore not found
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	{
		v := viper.New()
		v.SetConfigName(strings.TrimSuffix(config.RewriteFileName, filepath.Ext(config.RewriteFileName)))
		v.AddConfigPath(configPath)
		v.SetConfigType("yaml")
		err := v.ReadInConfig()
		if err == nil {
			viper.MergeConfigMap(v.AllSettings())
		}
	}

	Config = *config.Conf

	// init APM EVN Config
	if config.Conf.App.Middleware.APM.APMStartUp {
		os.Setenv("ELASTIC_APM_SERVER_URL", config.Conf.App.Middleware.APM.APMServerURL)
		apm.DefaultTracer.Transport, _ = transport.InitDefault()
	}

	// init logger
	logLevel := env.Env.LogLevel
	if customLogLevel != "" {
		logLevel = env.LogLevelMap[customLogLevel]
	}

	opts := make([]logging.LogConfigOption, 0)
	if logLevel == int(zapcore.DebugLevel) {
		opts = append(opts, logging.WithLogLevel(logging.DebugLevel))
		opts = append(opts, logging.WithReleaseLevel(logging.DevelopmentLevel))
	}

	if config.Conf.App.Middleware.Logger.UseFile {
		opts = append(opts,
			logging.WithUseConsole(false),
			logging.WithLogPath(ensureLogPath()),
			logging.WithMaxAge(config.Conf.App.Middleware.Logger.MaxAge),
			logging.WithMaxSize(config.Conf.App.Middleware.Logger.MaxSize),
			logging.WithMaxBackups(config.Conf.App.Middleware.Logger.MaxBackups),
		)
	}

	logger, err = logging.NewLogger(opts...)
	if err != nil {
		return fmt.Errorf("new logger failed, %w", err)
	}
	if _, err = logging.SetDefault(logging.WithLogger(logger)); err != nil {
		return fmt.Errorf("set default logger failed, %w", err)
	}

	// the logger.log_dir is optional
	if d := config.Conf.App.Middleware.Logger.LogDir; len(d) != 0 {
		file.TouchDir(d)
		_ = flag.Set("log_dir", d)
	}

	if version.ShouldLogVersion() {
		version.LogVersion()
		os.Exit(0)
	}

	logger.Debug("logLevel: " + env.LogLevelName(env.Env.LogLevel))
	logger.Debug(logMode, logConfig)
	configLoaded = true
	return nil
}

var (
	telemetryOnce sync.Once
	telemetryErr  error
)

// InitTelemetry 初始化 tracing 与 gRPC 客户端池。它必须在任何 gRPC
// server 构造之前调用：driver 在构造时读取拦截器列表。
func InitTelemetry() error {
	telemetryOnce.Do(func() {
		if _, err := middleware.InitOTEL(config.Conf); err != nil {
			telemetryErr = err
			return
		}
		initGRPCClient()
	})

	return telemetryErr
}

var (
	migrateOnce sync.Once
	migrateErr  error
)

// Migrate 执行数据库自动迁移。Application 在启动插件之前调用它。
func Migrate() error {
	migrateOnce.Do(func() {
		migrateErr = doMigration()
	})

	return migrateErr
}

func initGRPCClient() {
	grpc_boot.InitClient(&config.Conf.App.Middleware.GRPC.Client)
}

const (
	defaultLogDir      = "log"
	defaultLogFilename = "custom.log"
)

func ensureLogPath() string {
	logDir := defaultLogDir
	if config.Conf.App.Middleware.Logger.LogDir != "" {
		logDir = config.Conf.App.Middleware.Logger.LogDir
	}

	logFilename := defaultLogFilename
	if config.Conf.App.Name != "" {
		logFilename = fmt.Sprintf("%s.log", config.Conf.App.Name)
	}

	return filepath.Join(logDir, logFilename)
}

func doMigration() error {
	if !config.Conf.App.DBMigration.AutoMigrate {
		return nil
	}

	defaultMysql, exist := config.Conf.App.Middleware.Mysql["default"]
	if !exist {
		return fmt.Errorf("auto migration enabled but middleware.mysql.default is not configured")
	}

	if config.Conf.App.Name == "" {
		return fmt.Errorf("auto migration enabled but app.name is empty")
	}

	if defaultMysql.Dsn == "" {
		return fmt.Errorf("auto migration enabled but middleware.mysql.default.dsn is empty")
	}

	return migration.Run(defaultMysql.Dsn, config.Conf.App.Name)
}
