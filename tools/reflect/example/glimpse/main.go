package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/ez4bk/ezboot/snowflake"
	"github.com/ez4bk/ezboot/tools/reflect"
)

var dsn string

func init() {
	flag.StringVar(&dsn, "dsn", "", "connection string of the database")
	flag.Parse()

	if envDsn := os.Getenv("MYSQL_DSN"); len(envDsn) != 0 {
		dsn = envDsn
	}

	rand.Seed(time.Now().UnixNano())
}

func main() {
	if len(dsn) == 0 {
		fmt.Println("error: unknown dsn for database")
		os.Exit(1)
	}

	r, err := reflect.New(dsn, reflect.WithPolicy(&reflect.Policy{
		Tables: map[string]*reflect.TablePolicy{
			"user": {
				Alias: "AdminUser",
				Columns: map[string]*reflect.ColumnPolicy{
					"id": {
						Alias:     "user_id",
						TypeValue: snowflake.ID(0),
					},
					"username": {
						TypeValue: &reflect.QualifiedIdentifier{
							Identifier: "[]byte",
						},
					},
				},
				DAOPolicy: &reflect.DAOPolicy{
					Constraint: map[string]*reflect.DAOListConstraint{
						"id": {
							OptionalSkip: 0,
						},
						"username": {
							Likely: true,
						},
						"password": {
							Nullable: true,
						},
					},
				},
				CachePolicy: &reflect.CachePolicy{
					Prefix:  "kw:test:",
					Expired: 3 * time.Minute,
				},
			},
		},
	}))
	if err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}

	tables := os.Args[1:]
	if len(tables) == 0 {
		_ = r.Visit(func(table string) error {
			tables = append(tables, table)
			return nil
		})
	}

	for _, table := range tables {
		schema := r.Schema(table)
		if len(schema.PrimaryKeys) == 0 {
			continue
		}

		var daoOptions []reflect.DAORenderOption
		// for _, column := range schema.Columns() {
		//	if column.SQLType.IsText() {
		//		if rand.Int()%2 == 0 { // likely
		//			daoOptions = append(daoOptions, reflect.WithDAOListerConstraintLikely(column.Name, rand.Int()%2 == 0))
		//		} else { // enum
		//			daoOptions = append(daoOptions, reflect.WithDAOListConstraintEnum(column.Name, "example.package/api.MoneyType_UNKNOWN", rand.Int()%2 == 0))
		//		}
		//	} else if column.SQLType.IsNumeric() {
		//		daoOptions = append(daoOptions, reflect.WithDAOListConstraintRequire(column.Name))
		//	}
		// }

		err = r.Render(table, os.Stdout,
			reflect.WithFieldTagMapper(reflect.JsonTagMapper()),
			reflect.WithFieldTagMapper(reflect.YamlTagMapper()),
			reflect.WithFieldTagMapper(reflect.XORMTagMapper()),
			reflect.WithFieldTypeMapperByName("id", snowflake.ID(0)),
			reflect.WithDAORender(os.Stdout, daoOptions...),
		)
		if err != nil {
			fmt.Printf("error: %s\n", err)
			os.Exit(1)
		}
	}
}
