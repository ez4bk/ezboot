package migration

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"

	"github.com/ez4bk/ezboot/internal/logging"
)

const mysqlScheme = "mysql://"

// Run 以 fsys 为迁移源，将数据库升级到最新版本。
//
// fsys 的根目录必须是迁移文件所在目录，例如 //go:embed *.sql 生成的
// embed.FS 可以直接传入。
func Run(dsn, appName string, fsys fs.FS) error {
	if dsn == "" {
		return fmt.Errorf("migration dsn cannot be empty")
	}

	if appName == "" {
		return fmt.Errorf("migration app name cannot be empty")
	}

	dsn, err := completionDSN(dsn, appName)
	if err != nil {
		return fmt.Errorf("complete migration dsn failed: %w", err)
	}

	if fsys == nil {
		return fmt.Errorf("migration fs cannot be nil: inject the embedded migrations with ezboot.WithMigrations")
	}

	driver, err := iofs.New(fsys, ".")
	if err != nil {
		return fmt.Errorf(
			"load migration schemas failed: migration files must be at the root of the provided filesystem: %w",
			err,
		)
	}

	// TODO: Strictly validate migration filenames, paired directions, and version continuity before connecting.
	migrator, err := migrate.NewWithSourceInstance("iofs", driver, dsn)
	if err != nil {
		sourceErr := driver.Close()
		return errors.Join(
			fmt.Errorf("create migration failed: %w", err),
			wrapCloseError(sourceErr),
		)
	}

	migrator.Log = &logger{underlying: logging.Default()}

	return run(migrator)
}

func completionDSN(dsn, appName string) (string, error) {
	rawDSN := strings.TrimPrefix(dsn, mysqlScheme)
	config, err := mysql.ParseDSN(rawDSN)
	if err != nil {
		return "", err
	}

	if config.Params == nil {
		config.Params = make(map[string]string)
	}
	config.Params["x-migrations-table"] = fmt.Sprintf("%s_schema_version", appName)

	return mysqlScheme + config.FormatDSN(), nil
}

func run(migrator *migrate.Migrate) (err error) {
	defer func() {
		sourceErr, databaseErr := migrator.Close()
		err = errors.Join(
			err,
			wrapCloseError(sourceErr),
			wrapCloseError(databaseErr),
		)
	}()

	err = migrator.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}

	var dirtyErr migrate.ErrDirty
	if errors.As(err, &dirtyErr) {
		return fmt.Errorf(
			"migration state is dirty at version %d; inspect the failed SQL and repair the database manually before restarting: %w",
			dirtyErr.Version,
			err,
		)
	}

	return err
}

func wrapCloseError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close migration resource failed: %w", err)
}

type logger struct {
	underlying *logging.Logger
}

func (l *logger) Printf(format string, v ...any) {
	l.underlying.Infof(format, v...)
}

func (l *logger) Verbose() bool {
	return l.underlying.Desugar().Core().Enabled(zap.DebugLevel)
}
