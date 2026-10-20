package reflect

import (
	"testing"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

// conversionValue 模拟实现了 xorm convert.Conversion 的自定义类型（如 geopoint）
type conversionValue struct{ X, Y float64 }

func (conversionValue) ToDB() ([]byte, error) { return nil, nil }
func (*conversionValue) FromDB([]byte) error  { return nil }

func TestXORMTagMapperOmitsTypeNameForConversionTypes(t *testing.T) {
	mapper := XORMTagMapper()

	t.Run("Conversion 类型不写 SQL 类型名", func(t *testing.T) {
		field := &Field{
			Column: &schemas.Column{Name: "location", SQLType: schemas.SQLType{Name: "point"}, Nullable: false},
			Name:   "Location",
			Type:   &FieldType{Value: conversionValue{}},
		}
		got := mapper.Content(field)
		require.Equal(t, "'location' not null", got)
		require.NotContains(t, got, "point", "Conversion 类型的 tag 不能带 SQL 类型名")
	})

	t.Run("普通类型保留 SQL 类型名", func(t *testing.T) {
		field := &Field{
			Column: &schemas.Column{Name: "title", SQLType: schemas.SQLType{Name: "varchar", DefaultLength: 120}, Nullable: false},
			Name:   "Title",
			Type:   &FieldType{Value: ""},
		}
		require.Equal(t, "'title' varchar(120) not null", mapper.Content(field))
	})
}

func TestIsConversionType(t *testing.T) {
	require.True(t, isConversionType(&Field{Type: &FieldType{Value: conversionValue{}}}))
	require.False(t, isConversionType(&Field{Type: &FieldType{Value: ""}}))
	require.False(t, isConversionType(&Field{Type: nil}))
	require.False(t, isConversionType(nil))
}
