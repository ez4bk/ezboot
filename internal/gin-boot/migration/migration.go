package migration

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"

	"github.com/ez4bk/ezboot/internal/logging"
)

const schemasDirectory = "schemas"
const mysqlScheme = "mysql://"

func Run(dsn, appName string) error {
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

	source, err := iofs.New(os.DirFS(schemasDirectory), ".")
	if err != nil {
		return fmt.Errorf("load migration schemas from %q failed: %w", schemasDirectory, err)
	}

	// TODO: Strictly validate migration filenames, paired directions, and version continuity before connecting.
	migrator, err := migrate.NewWithSourceInstance("iofs", source, dsn)
	if err != nil {
		sourceErr := source.Close()
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
