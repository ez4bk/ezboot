package model

import (
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"

	"github.com/ez4bk/ezboot/snowflake"
	"github.com/ez4bk/ezboot/tools/reflect"
)

func init() {
	options.envDsn = os.Getenv("EZBOOT_MYSQL_DSN")
}

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "generate the model definition from database schema",
		PreRunE: func(*cobra.Command, []string) error {
			if len(options.dsn) == 0 {
				options.dsn = options.envDsn
			}
			if len(options.dsn) == 0 {
				return errors.New("unknown database")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = chooseTables()
			}

			r, err := reflect.New(options.dsn)
			if err != nil {
				return err
			}

			return generate(r, args...)
		},
	}

	cmd.Flags().StringVar(&options.dsn, "dsn", "", "the dsn for database")
	cmd.Flags().StringVar(&options.dir, "dir", "", "where to save the generated model file")

	return cmd
}

var options struct {
	envDsn string
	dsn    string
	dir    string

	engine *xorm.Engine
	tables map[string]*schemas.Table
}

func chooseTables() (ts []string) {
	var names []string
	for _, table := range options.tables {
		names = append(names, table.Name)
	}

	prompt := &survey.MultiSelect{
		Message: "Which tables do you want to generate:",
		Options: names,
	}

	_ = survey.AskOne(prompt, &ts, survey.WithPageSize(10))
	return
}

func generate(r *reflect.Reflection, tables ...string) error {
	for _, table := range tables {
		filename := filepath.Join(options.dir, strcase.ToSnake(table)+".go")
		fp, err := os.Create(filename)
		if err != nil {
			return err
		}

		renderOptions := []reflect.RenderOption{
			reflect.WithFieldTypeMapperByName("id", snowflake.ID(0)),
			reflect.WithFieldTagMapper(reflect.JsonTagMapper()),
			reflect.WithFieldTagMapper(reflect.XORMTagMapper()),
		}

		if err = r.Render(table, fp, renderOptions...); err != nil {
			_ = fp.Close()
			return err
		}
		_ = fp.Close()
	}

	return nil
}
