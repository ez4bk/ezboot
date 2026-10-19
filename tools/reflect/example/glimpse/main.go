package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/ez4bk/ezboot/snowflake"

	"xorm.io/xorm/schemas"

	"github.com/ez4bk/ezboot/tools/reflect"
)

var (
	dsn  string
	mock bool
)

func init() {
	flag.StringVar(&dsn, "dsn", "", "connection string of the database")
	flag.BoolVar(&mock, "mock", true, "render an offline mock table instead of reading the database")
	flag.Parse()

	if envDsn := os.Getenv("MYSQL_DSN"); len(envDsn) != 0 {
		dsn = envDsn
	}

	rand.Seed(time.Now().UnixNano())
}

func main() {
	policy := &reflect.Policy{
		Tables: map[string]*reflect.TablePolicy{
			"user": {
				Alias: "AdminUser",
				PagerPolicy: &reflect.PagerPolicy{
					Type: reflect.PagerTypeToken,
				},
				Columns: map[string]*reflect.ColumnPolicy{
					"id": {
						Alias:     "user_id",
						TypeValue: snowflake.ID(0),
					},
					"username": {
						TypeValue: "string",
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
	}

	var (
		r   *reflect.Reflection
		err error
	)
	if mock {
		r, err = reflect.NewWithTables(mockTables(), reflect.WithPolicy(policy))
	} else {
		if len(dsn) == 0 {
			fmt.Println("error: unknown dsn for database")
			os.Exit(1)
		}
		r, err = reflect.New(dsn, reflect.WithPolicy(policy))
	}
	if err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
	defer r.Close()

	tables := flag.Args()
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

// mockTables 构造离线预览使用的表结构
func mockTables() map[string]*schemas.Table {
	return map[string]*schemas.Table{
		"user": mockUserTable(),
	}
}

func mockUserTable() *schemas.Table {
	table := schemas.NewTable("user", nil)
	table.PrimaryKeys = []string{"id"}

	id := mockColumn("id", schemas.UnsignedBigInt, 20, false)
	id.IsPrimaryKey = true
	id.IsAutoIncrement = true
	table.AddColumn(id)

	table.AddColumn(mockColumn("username", schemas.Varchar, 64, false))
	table.AddColumn(mockColumn("password", schemas.Varchar, 255, false))
	table.AddColumn(mockColumn("created_time", schemas.DateTime, 0, false))
	table.AddColumn(mockColumn("update_time", schemas.DateTime, 0, false))
	deleted := mockColumn("deleted", schemas.TinyInt, 1, false)
	deleted.SQLType.DefaultLength = 1
	table.AddColumn(deleted)
	table.AddColumn(mockColumn("_version", schemas.Int, 0, false))

	return table
}

func mockColumn(name, sqlType string, length int64, nullable bool) *schemas.Column {
	return schemas.NewColumn(name, name, schemas.SQLType{Name: sqlType}, length, 0, nullable)
}
