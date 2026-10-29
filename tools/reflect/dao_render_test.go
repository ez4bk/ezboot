package reflect

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

func renderExampleDAO(t *testing.T, withDefault bool) string {
	t.Helper()

	r := &render{
		table: &schemas.Table{Name: "example"},
		renderModelIdentifier: &QualifiedIdentifier{
			Import:     "example.com/models",
			Identifier: "Example",
		},
		renderModelFields: map[string]*FieldQualifiedIdentifier{
			"id": {
				FieldName:  "Id",
				PrimaryKey: true,
				Identifier: &QualifiedIdentifier{
					Import:     "github.com/ez4bk/ezboot/snowflake",
					Identifier: "ID",
				},
			},
			"deleted": {
				FieldName:  "Deleted",
				Identifier: &QualifiedIdentifier{Identifier: "bool"},
			},
		},
	}

	dr, err := newDAORender(r,
		WithDAOPkgName("dao"),
		WithDAOCache("example:", time.Hour),
		WithDAOWithDefault(withDefault),
	)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, dr.Render(&buf))

	_, err = parser.ParseFile(token.NewFileSet(), "example.dao.go", buf.Bytes(), 0)
	require.NoError(t, err, "generated file must be valid Go")

	return buf.String()
}

func TestDAORenderWithDefault(t *testing.T) {
	out := renderExampleDAO(t, true)

	require.Contains(t, out, "with.DefaultSession(ctx, func(db *xorm.Session) error")
	require.Contains(t, out, "with.DefaultRedis(ctx, func(cache *redis.Client) error")
	require.Contains(t, out, "func NewExampleDAO(log *ezboot.Logger) (*ExampleDAO, error)")
	require.NotContains(t, out, "dao.cache")
	require.NotContains(t, out, "*xorm.Engine")
}

func TestDAORenderWithInjectedClients(t *testing.T) {
	out := renderExampleDAO(t, false)

	require.Contains(t, out, "func NewExampleDAO(log *ezboot.Logger, db *xorm.Engine, cache *redis.Client) (*ExampleDAO, error)")
	require.Contains(t, out, "dao.withSession(ctx, func(db *xorm.Session) error")
	require.Contains(t, out, "dao.cache.Get(")
	require.Contains(t, out, "dao.cache.Set(")
	require.Contains(t, out, "dao.cache.Del(")
	require.Contains(t, out, "func (dao *ExampleDAO) withSession(ctx context.Context, f func(db *xorm.Session) error) error")
	require.NotContains(t, out, "with.DefaultSession")
	require.NotContains(t, out, "with.DefaultRedis")
	require.False(t, strings.Contains(out, `"github.com/ez4bk/ezboot/with"`))
}
