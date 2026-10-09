package reflect

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/dave/jennifer/jen"
	"github.com/iancoleman/strcase"
	"xorm.io/xorm/schemas"
)

const (
	// DefaultDAOPkgName 默认的访问层包名
	DefaultDAOPkgName = "dao"

	vCtx = "ctx"
	vDb  = "db"
)

var (
	idErrorNew       = &QualifiedIdentifier{Import: "errors", Identifier: "New"}
	idContext        = &QualifiedIdentifier{Import: "context", Identifier: "Context"}
	idXormSession    = &QualifiedIdentifier{Import: "xorm.io/xorm", Identifier: "Session"}
	idEZBootLogger   = &QualifiedIdentifier{Import: "github.com/ez4bk/ezboot", Identifier: "Logger"}
	idWithSession    = &QualifiedIdentifier{Import: "github.com/ez4bk/ezboot/with", Identifier: "DefaultSession"}
	idLogWarn        = &QualifiedIdentifier{Import: "github.com/ez4bk/ezboot/elog", Identifier: "Warnw"}
	idEXormPager     = &QualifiedIdentifier{Import: "github.com/ez4bk/ezboot/exorm", Identifier: "Pager"}
	idEXormNew       = &QualifiedIdentifier{Import: "github.com/ez4bk/ezboot/exorm", Identifier: "New"}
	idRedisClient    = &QualifiedIdentifier{Import: "github.com/go-redis/redis", Identifier: "Client"}
	idJsonMarshal    = &QualifiedIdentifier{Import: "encoding/json", Identifier: "Marshal"}
	idJsonUnmarshal  = &QualifiedIdentifier{Import: "encoding/json", Identifier: "Unmarshal"}
	idTimeDuration   = &QualifiedIdentifier{Import: "time", Identifier: "Duration"}
	idUtilsToString  = &QualifiedIdentifier{Import: "github.com/ez4bk/ezboot/utils", Identifier: "ToString"}
	namedContext     = jen.Id(vCtx).Qual(idContext.Import, idContext.Identifier)
	namedXormSession = jen.Id(vDb).Op("*").Qual(idXormSession.Import, idXormSession.Identifier)
)

// daoRender 数据访问层的代码生成器
type daoRender struct {
	table *schemas.Table

	pkgName    string
	headerLine string

	modelIdentifier *QualifiedIdentifier
	modelFields     map[string]*FieldQualifiedIdentifier

	structNameMapper      StringMapper
	constructorNameMapper StringMapper

	listerFields []*ListConstraint

	useCache     bool
	cachePrefix  string
	cacheExpired time.Duration
}

// Render 将数据访问层的东西渲染到 w 中
func (r *daoRender) Render(w io.Writer) error {
	g := jen.NewFilePath(r.pkgName)

	g.HeaderComment(r.headerLine)
	g.Line()

	g.Var().DefsFunc(r.Declares)

	typename := r.structNameMapper.Mapping(r.modelIdentifier.Identifier)
	g.Commentf("%s manage the %s model", typename, r.table.Name)
	g.Type().Id(typename).StructFunc(func(gp *jen.Group) {
		gp.Id("log").Op("*").Qual(idEZBootLogger.Import, idEZBootLogger.Identifier)
		// 如果该表启用缓存
		if r.useCache {
			gp.Id("cache").Op("*").Qual(idRedisClient.Import, idRedisClient.Identifier)
		}
	})

	namedPtrTypename := jen.Id("dao").Op("*").Id(typename)
	if len(r.pkName()) != 0 {
		r.GetMethod(g, namedPtrTypename)
		r.ExistsMethod(g, namedPtrTypename)
		r.ListMethod(g, namedPtrTypename)
		r.CreateMethod(g, namedPtrTypename)
		r.UpdateMethod(g, namedPtrTypename)
		if _, ok := r.modelFields["deleted"]; ok {
			r.DeleteMethod(g, namedPtrTypename)
		}
		r.DestroyMethod(g, namedPtrTypename)
	}

	r.Constructor(g, typename, jen.Op("*").Id(typename))

	r.jenImportPkg(g, idErrorNew)
	r.jenImportPkg(g, idContext)
	r.jenImportPkg(g, idXormSession)
	r.jenImportPkg(g, idEZBootLogger)
	r.jenImportPkg(g, idWithSession)
	r.jenImportPkg(g, idLogWarn)
	r.jenImportPkg(g, idEXormPager)
	r.jenImportPkg(g, idEXormNew)
	r.jenImportPkg(g, idRedisClient)
	r.jenImportPkg(g, idJsonMarshal)
	r.jenImportPkg(g, idJsonUnmarshal)
	r.jenImportPkg(g, idTimeDuration)
	r.jenImportPkg(g, idUtilsToString)

	return g.Render(w)
}

// Declares 声明模型访问一些常见的错误和常量
func (r *daoRender) Declares(gp *jen.Group) {
	gp.Id(r.errModelNotFound()).Op("=").Add(idErrorNew.AsCode()).
		Call(jen.Lit(fmt.Sprintf("dao: %s not found", r.table.Name)))

	gp.Line()
	gp.Id(r.predefineNilModel()).Op("=").Params(jen.Op("*").Add(r.modelIdentifier.AsCode())).Params(jen.Nil())
}

// GetMethod 创建一个获取模型方法
func (r *daoRender) GetMethod(g *jen.File, pTypename *jen.Statement) {
	outParams := jen.List(jen.Op("*").Add(r.modelIdentifier.AsCode()), jen.Error())
	inParams := jen.List(namedContext, jen.Id("id").Add(r.pkIdentifier().AsCode()), jen.Id("withDeleted").Op("...").Id("bool"))

	g.Comment("Get retrieves the specified model from database by pk")
	g.Func().Params(pTypename).Id("Get").Params(inParams).Params(outParams).BlockFunc(func(gp *jen.Group) {
		modelPtr := r.jenVarModel(gp)
		op := ":="
		// 如果启用了缓存，就先从缓存中读取
		if r.useCache {
			op = "="
			gp.Id("data").Op(",").Err().Op(":=").Id("dao").Dot("cache").Dot("Get").Call(r.jenModelIdCreateKey(false)).Dot("Bytes").Call()
			gp.If(jen.Err().Op("==").Nil()).Block(
				jen.Err().Op("=").Qual(idJsonUnmarshal.Import, idJsonUnmarshal.Identifier).Call(jen.Id("data"), modelPtr),
				jen.If(jen.Err().Op("==").Nil()).Block(
					jen.Return(modelPtr.Clone().Op(",").Nil()),
				),
			)
			gp.Line()
		}

		r.jenWithDefaultSession(gp, func(gp *jen.Group) {
			condConstraint := []jen.Code{
				jen.Id("len").Call(jen.Id("withDeleted")).Op("==").Lit(0).Op("||").
					Op("!").Id("withDeleted").Index(jen.Lit(0)),
				jen.Lit("deleted = ?"),
				jen.Lit(false),
			}

			gp.Id("exists").Op(",").Err().Op(":=").Add(idEXormNew.AsCode()).
				Call(jen.Id("db")).Op(".").Line().
				Id("Cond").Call(condConstraint...).Op(".").Line().
				Id("Raw").Call().Op(".").Line().
				Id("ID").Call(jen.Id("id")).Op(".").Line().
				Id("Get").Call(modelPtr)

			r.jenIfErrNotNil(gp, func(gp *jen.Group) {
				r.jenReturnErrWrap(gp, "failed to get %s by id form database", r.table.Name)
			})

			gp.If(jen.Op("!").Id("exists")).BlockFunc(func(gp *jen.Group) {
				gp.Return(jen.Id(r.errModelNotFound()))
			})

			gp.Line()
			gp.Return(jen.Nil())
		}, op) // with.DefaultSession

		gp.Line()
		r.jenIfErrNotNil(gp, func(gp *jen.Group) {
			gp.Return(jen.Nil(), jen.Err())
		})

		if r.useCache {
			gp.Line().Id("value").Op(",").Err().Op(":=").Qual(idJsonMarshal.Import, idJsonMarshal.Identifier).Call(jen.Id("model"))
			gp.If(jen.Err().Op("==").Nil()).Block(
				jen.Err().Op("=").Id("dao").Dot("cache").Dot("Set").Call(r.jenModelIdCreateKey(false), jen.Id("string").Call(jen.Id("value")), jen.Qual(idTimeDuration.Import, idTimeDuration.Identifier).Call(jen.Id(strconv.FormatInt(int64(r.cacheExpired), 10)))).Dot("Err").Call(),
				jen.If(jen.Err().Op("!=").Nil()).Block(
					jen.Id("dao").Dot("log").Dot("Warnw").Call(jen.Lit("failed to set redis cache"), jen.Lit("error"), jen.Err()),
				),
			)
			gp.Line()
		}

		gp.Return(modelPtr, jen.Nil())
	}) // Get

	g.Line()
}

// ExistsMethod 创建一个检查用户是否存在的方法
func (r *daoRender) ExistsMethod(g *jen.File, pTypename *jen.Statement) {
	outParams := jen.List(jen.Id("exists").Id("bool"), jen.Err().Error())
	inParams := jen.List(namedContext, jen.Id("id").Add(r.pkIdentifier().AsCode()))

	g.Comment("Exists returns true when specified model exists in the database, false otherwise")
	g.Func().Params(pTypename).Id("Exists").Params(inParams).Params(outParams).BlockFunc(func(gp *jen.Group) {
		r.jenWithDefaultSession(gp, func(gp *jen.Group) {
			gp.Id("exists").Op(",").Err().Op("=").Id("db").Op(".").Line().
				Id("Table").Call(jen.Id(r.predefineNilModel()).Dot("TableName").Call()).Op(".").Line().
				Id("ID").Call(jen.Id("id")).Op(".").Line().
				Id("Where").Call(jen.Lit("deleted = ?"), jen.Lit(false)).Op(".").Line().
				Id("Exist").Call()

			r.jenReturnErrWrap(gp, "failed to check %s exists by id form database", r.table.Name)
		}, "=") // with.DefaultSession

		gp.Return()
	}) // Exists

	g.Line()
}

// ListMethod 列出模型列表的方法
func (r *daoRender) ListMethod(g *jen.File, pTypename *jen.Statement) {
	typename := r.jenListParamsType(g)

	sliceModelType := jen.Id("list").Index().Op("*").Add(r.modelIdentifier.AsCode())
	outParams := jen.List(sliceModelType, jen.Id("total").Id("int64"), jen.Err().Error())
	inParams := jen.List(namedContext, jen.Id("params").Op("*").Id(typename))

	g.Comment("List returns the specified models from database by params")
	g.Func().Params(pTypename).Id("List").Params(inParams).Params(outParams).BlockFunc(func(gp *jen.Group) {
		r.jenWithDefaultSession(gp, func(gp *jen.Group) {
			stmt := gp.Id("total").Op(",").Err().Op("=").Add(idEXormNew.AsCode()).
				Call(jen.Id("db")).Op(".").Line()

			for _, constraint := range r.listerFields {
				mf := r.modelFields[constraint.fieldName]
				if constraint.optional || constraint.nullable {
					field := jen.Id("params").Dot(mf.FieldName)
					if constraint.nullable {
						stmt.Id("Nullable").Call(constraint.jenConditions(field, mf)...).Op(".").Line()
					} else {
						stmt.Id("Cond").Call(constraint.jenConditions(field, mf)...).Op(".").Line()
					}
				}
			}

			stmt.
				Id("Limit").Call(jen.Id("params")).Op(".").Line().
				Id("Raw").Call().Op(".").Line()

			var hasDeleted bool
			for _, constraint := range r.listerFields {
				hasDeleted = hasDeleted || constraint.fieldName == "deleted"

				mf := r.modelFields[constraint.fieldName]
				if !constraint.optional && !constraint.nullable {
					field := jen.Id("params").Dot(mf.FieldName)
					stmt.Id("Where").Call(constraint.jenConditions(field, mf)[1:]...).Op(".").Line()
				}
			}

			if !hasDeleted {
				stmt.
					Id("Where").Call(jen.Lit("deleted = ?"), jen.Lit(false)).Op(".").Line()
			}

			stmt.
				Id("OrderBy").Call(jen.Lit(fmt.Sprintf("%s desc", r.pkName()))).Op(".").Line().
				Id("FindAndCount").Call(jen.Op("&").Id("list"))
			r.jenIfErrNotNil(gp, func(gp *jen.Group) {
				r.jenReturnErrWrap(gp, "failed to list %s from database", r.table.Name)
			})

			gp.Line()
			gp.Return(jen.Nil())
		}, "=")

		gp.Line()
		gp.Return()
	}) // List

	g.Line()
}

// jenListParamsType 返回列出模型列表的类型名称
func (r *daoRender) jenListParamsType(g *jen.File) string {
	typename := fmt.Sprintf("List%sParams", r.modelIdentifier.Identifier)

	g.Commentf("%s represents the params to list models", typename)
	g.Type().Id(typename).StructFunc(func(gp *jen.Group) {
		gp.Add(idEXormPager.AsCode())

		gp.Line()
		for _, constraint := range r.listerFields {
			var attributes []string
			if constraint.optional {
				attributes = append(attributes, "optional")
			} else {
				attributes = append(attributes, "required")
			}

			if constraint.likely {
				attributes = append(attributes, "likely")
			}

			field := r.modelFields[constraint.fieldName]
			stmt := gp.Id(field.FieldName)
			if constraint.nullable {
				stmt = stmt.Op("*")
			}
			stmt.Add(field.Identifier.AsCode()).Comment(strings.Join(attributes, ", "))
		}
	})

	return typename
}

// CreateMethod 创建一个生成模型方法
func (r *daoRender) CreateMethod(g *jen.File, pTypename *jen.Statement) {
	closureName := "f"
	closureParams := jen.Op("*").Add(r.modelIdentifier.AsCode())
	outParams := jen.List(r.pkIdentifier().AsCode(), jen.Error())
	inParams := jen.List(namedContext, jen.Id(closureName).Func().Params(closureParams).Error())

	g.Comment("Create creates and insert the new model into database")
	g.Func().Params(pTypename).Id("Create").Params(inParams).Params(outParams).BlockFunc(func(gp *jen.Group) {
		modelPtr := r.jenVarModel(gp)
		r.jenWithDefaultSession(gp, func(gp *jen.Group) {
			gp.IfFunc(func(gp *jen.Group) {
				gp.Err().Op(":=").Id(closureName).Call(modelPtr).Op(";").
					Err().Op("!=").Nil().Block(jen.Return(jen.Err()))
			}) // if

			gp.Line()
			gp.Op("_").Op(",").Err().Op(":=").Id(vDb).Dot("Insert").Call(modelPtr)
			r.jenIfErrNotNil(gp, func(gp *jen.Group) {
				r.jenReturnErrWrap(gp, "failed to insert the %s into database", r.table.Name)
			})

			gp.Line()
			gp.Return(jen.Nil())
		}, ":=") // with.DefaultSession

		gp.Return(jen.Id("model").Dot(r.modelFields[r.pkName()].FieldName), jen.Err())
	}) // Create

	g.Line()
}

// UpdateMethod 创建一个更新模型的方法
func (r *daoRender) UpdateMethod(g *jen.File, pTypename *jen.Statement) {
	inParams := jen.List(
		namedContext,
		jen.Id("model").Op("*").Add(r.modelIdentifier.AsCode()),
		jen.Id("cols").Op("...").Id("string"),
	)

	g.Comment("Update updates the model from database")
	g.Func().Params(pTypename).Id("Update").Params(inParams).Error().BlockFunc(func(gp *jen.Group) {
		r.jenWithDefaultSession(gp, func(gp *jen.Group) {
			gp.Op("_").Op(",").Err().Op(":=").Id("db").Op(".").Line().
				Id("ID").Call(jen.Id("model").Dot(r.modelFields[r.pkName()].FieldName)).Op(".").Line().
				Id("MustCols").Call(jen.Id("cols").Op("...")).Op(".").Line().
				Id("Where").Call(jen.Lit("deleted = ?"), jen.Lit(false)).Op(".").Line().
				Id("Update").Call(jen.Id("model"))

			gp.Line()
			r.jenReturnErrWrap(gp, "failed to update the %s in the database", r.table.Name)
		}, ":=") // with.DefaultSession

		// 加一段删除缓存的逻辑
		if r.useCache {
			r.jenDeleteCache(gp, true)
		}

		gp.Return(jen.Err())
	}) // Exists

	g.Line()
}

// DeleteMethod 软删除一个数据对象
func (r *daoRender) DeleteMethod(g *jen.File, pTypename *jen.Statement) {
	outParams := jen.List(jen.Id("bool"), jen.Error())
	deletedStruct := jen.Values(jen.Dict{jen.Id(r.modelFields["deleted"].FieldName): jen.Lit(true)})
	inParams := jen.List(namedContext, jen.Id("id").Add(r.pkIdentifier().AsCode()))

	g.Comment("Delete deletes the model by update the deleted field and returns whose whether succeed")
	g.Func().Params(pTypename).Id("Delete").Params(inParams).Params(outParams).BlockFunc(func(gp *jen.Group) {
		gp.Var().Id("deleted").Id("bool")
		r.jenWithDefaultSession(gp, func(gp *jen.Group) {
			gp.Id("affects").Op(",").Err().Op(":=").Add(idEXormNew.AsCode()).Call(jen.Id("db")).Op(".").Line().
				Id("NoCheckVersion").Call().Op(".").Line().
				Id("Raw").Call().Op(".").Line().
				Id("ID").Call(jen.Id("id")).Op(".").Line().
				Id("Where").Call(jen.Lit("deleted = ?"), jen.Lit(false)).Op(".").Line().
				Id("UseBool").Call(jen.Lit("deleted")).Op(".").Line().
				Id("Update").Call(jen.Op("&").Add(r.modelIdentifier.AsCode()).Add(deletedStruct))

			gp.Line()
			gp.Id("deleted").Op("=").Id("affects").Op(">").Lit(0)
			r.jenReturnErrWrap(gp, "failed to soft delete the %s in the database", r.table.Name)
		}, ":=") // with.DefaultSession

		// 加一段删除缓存的逻辑
		if r.useCache {
			r.jenDeleteCache(gp, false)
		}

		gp.Return(jen.Id("deleted"), jen.Err())
	}) // Exists

	g.Line()
}

// DestroyMethod 销毁一个数据对象
func (r *daoRender) DestroyMethod(g *jen.File, pTypename *jen.Statement) {
	outParams := jen.List(jen.Id("bool"), jen.Error())
	inParams := jen.List(namedContext, jen.Id("model").Op("*").Add(r.modelIdentifier.AsCode()))

	g.Comment("Destroy deletes the model from database and returns whose whether succeed")
	g.Func().Params(pTypename).Id("Destroy").Params(inParams).Params(outParams).BlockFunc(func(gp *jen.Group) {
		gp.Var().Id("deleted").Id("bool")
		r.jenWithDefaultSession(gp, func(gp *jen.Group) {
			gp.Id("affects").Op(",").Err().Op(":=").Id("db").
				Dot("Delete").Call(jen.Id("model"))

			gp.Line()
			gp.Id("deleted").Op("=").Id("affects").Op(">").Lit(0)
			r.jenReturnErrWrap(gp, "failed to hard delete the %s in the database", r.table.Name)
		}, ":=") // with.DefaultSession

		// 加一段删除缓存的逻辑
		if r.useCache {
			r.jenDeleteCache(gp, true)
		}

		gp.Return(jen.Id("deleted"), jen.Err())
	}) // Exists

	g.Line()
}

// Constructor 创建一个构造器方法
func (r *daoRender) Constructor(g *jen.File, typename string, pTypename *jen.Statement) {
	funcName := r.constructorNameMapper.Mapping(typename)
	outParams := jen.List(pTypename, jen.Error())
	inParams := jen.Id("log").Op("*").Add(idEZBootLogger.AsCode())
	if r.useCache {
		inParams = jen.List(inParams, jen.Id("cache").Op("*").Add(idRedisClient.AsCode()))
	}
	loggerWith := jen.List(r.jenLiterals("component", "dao", "dao", r.table.Name)...)

	g.Func().Id(funcName).Params(inParams).Params(outParams).BlockFunc(func(gp *jen.Group) {
		gp.ReturnFunc(func(gp *jen.Group) {
			gp.ListFunc(func(gp *jen.Group) {
				jd := jen.Dict{jen.Id("log"): jen.Id("log").Dot("With").Call(loggerWith)}
				if r.useCache {
					jd[jen.Id("cache")] = jen.Id("cache")
				}

				gp.Op("&").Id(typename).Values(jd) // dao
				gp.Nil()                           // error
			})
		})
	})

	g.Line()
}

// jenVarModel 生成一个模型声明语句
func (r *daoRender) jenVarModel(gp *jen.Group) *jen.Statement {
	modelVar := "model"
	gp.Var().Id(modelVar).Add(r.modelIdentifier.AsCode())
	return jen.Op("&").Id(modelVar)
}

// jenModelIdCreateKey 根据模型id类型来创建Key
func (r *daoRender) jenModelIdCreateKey(member bool) *jen.Statement {
	arg := jen.Id("id")
	if member {
		arg = jen.Id("model").Dot("Id")
	}
	return jen.Lit(r.cachePrefix).Op("+").Qual(idUtilsToString.Import, idUtilsToString.Identifier).Call(arg)
}

// jenIfErrNotNil 检查错误是否为空
func (r *daoRender) jenIfErrNotNil(gp *jen.Group, f func(*jen.Group)) *jen.Group {
	gp.IfFunc(func(gp *jen.Group) { // if
		gp.Err().Op("!=").Nil().BlockFunc(f) // err != nil
	}) // end if
	return gp
}

// jenWithDefaultSession 开启一个 xorm.Session 会话
func (r *daoRender) jenWithDefaultSession(gp *jen.Group, f func(*jen.Group), op string) *jen.Group {
	gp.Err().Op(op).Add(idWithSession.AsCode()).CallFunc(func(gp *jen.Group) {
		gp.Id(vCtx).Op(",").Func().Params(namedXormSession).Error().BlockFunc(f)
	}) // with.DefaultSession
	return gp
}

// jenReturnErrWrap 返回一个错误和日志
func (r *daoRender) jenReturnErrWrap(gp *jen.Group, msg string, args ...any) *jen.Group {
	gp.Return(jen.Add(idLogWarn.AsCode()).Call(
		jen.Id("dao").Dot("log"),
		jen.Err(),
		jen.Lit(fmt.Sprintf(msg, args...)),
	)) // elog.Warnw
	return gp
}

// jenDeleteCache 删除redis缓存
func (r *daoRender) jenDeleteCache(gp *jen.Group, member bool) *jen.Group {
	gp.Line()
	gp.Err().Op("=").Id("dao").Dot("cache").Dot("Del").Call(r.jenModelIdCreateKey(member)).Dot("Err").Call()
	r.jenErrNotNilLogRedisError(gp, "failed to delete redis cache")
	gp.Line()
	return gp
}

// jenErrNotNilLogRedisError 打印操作redis错误
func (r *daoRender) jenErrNotNilLogRedisError(gp *jen.Group, message string) *jen.Group {
	gp.If(jen.Err().Op("!=").Nil()).Block(jen.Id("dao").Dot("log").Dot("Warnw").Call(
		jen.Lit(message),
		jen.Lit("error"),
		jen.Err(),
	))
	return gp
}

// jenImportPkg 导入一个包并且丢弃重命名
func (r *daoRender) jenImportPkg(g *jen.File, identifier *QualifiedIdentifier) {
	if slash := strings.LastIndex(identifier.Import, "/"); slash != -1 {
		g.ImportName(identifier.Import, identifier.Import[slash+1:])
	}
}

// jenLiterals 批量的字面值设置
func (r *daoRender) jenLiterals(vs ...any) (cs []jen.Code) {
	for _, v := range vs {
		cs = append(cs, jen.Lit(v))
	}
	return
}

// errModelNotFound 模型找不到的错误定义
func (r *daoRender) errModelNotFound() string {
	return fmt.Sprintf("Err%sNotFound", r.modelIdentifier.Identifier)
}

// predefineNilModel 预定义的空模型变量
func (r *daoRender) predefineNilModel() string {
	return fmt.Sprintf("_%s", r.modelIdentifier.Identifier)
}

// pkIdentifier 返回主键的类型信息
func (r *daoRender) pkIdentifier() *QualifiedIdentifier {
	for _, v := range r.modelFields {
		if v.PrimaryKey {
			return v.Identifier
		}
	}
	return nil
}

// pkName 返回主键名称
func (r *daoRender) pkName() string {
	for k, v := range r.modelFields {
		if v.PrimaryKey {
			return k
		}
	}
	return ""
}

// newDAORender 创建一个数据访问层渲染器
func newDAORender(r *render, options ...DAORenderOption) (*daoRender, error) {
	dr := &daoRender{
		table: r.table,

		pkgName:    DefaultDAOPkgName,
		headerLine: DefaultHeaderLine,

		modelIdentifier: r.renderModelIdentifier,
		modelFields:     r.renderModelFields,

		structNameMapper:      DefaultDAOStructNameMapper(),
		constructorNameMapper: DefaultDAOConstructorNameMapper(),
	}

	for _, option := range options {
		if err := option(dr); err != nil {
			return nil, err
		}
	}

	return dr, nil
}

// WithDAOPkgName 指定生成文件的包名
func WithDAOPkgName(pkg string) DAORenderOption {
	return func(dr *daoRender) error {
		dr.pkgName = pkg
		return nil
	}
}

// WithDAOPkgPath 指定生成文件的所在路径
func WithDAOPkgPath(path string) DAORenderOption {
	return func(dr *daoRender) error {
		dr.pkgName = path
		return nil
	}
}

// DAORenderOption 数据访问层渲染器配置选项
type DAORenderOption func(*daoRender) error

// WithDAOStructNameMapper 数据访问层的结构体名称
func WithDAOStructNameMapper(mapper StringMapper) DAORenderOption {
	return func(dr *daoRender) error {
		dr.structNameMapper = mapper
		return nil
	}
}

// DefaultDAOStructNameMapper 默认的结构体名称映射器
func DefaultDAOStructNameMapper() StringMapper {
	return StringMapperFunc(func(s string) string {
		return strcase.ToCamel(s) + "DAO"
	})
}

// WithDAOConstructorNameMapper 数据访问层的结构体的构造器名称
func WithDAOConstructorNameMapper(mapper StringMapper) DAORenderOption {
	return func(dr *daoRender) error {
		dr.constructorNameMapper = mapper
		return nil
	}
}

// WithDAOCache 设置缓存所需的参数
func WithDAOCache(Prefix string, Expired time.Duration) DAORenderOption {
	return func(dr *daoRender) error {
		dr.useCache = true
		dr.cachePrefix = Prefix
		dr.cacheExpired = Expired
		return nil
	}
}

// DefaultDAOConstructorNameMapper 默认的数据访问层的结构体的构造器名称
func DefaultDAOConstructorNameMapper() StringMapper {
	return StringMapperFunc(func(s string) string {
		return "New" + s
	})
}

// ListConstraint 列出模型的约束配置
type ListConstraint struct {
	fieldName string // 数据库字段的名字

	skipValue any                  // 跳过时使用的值
	enumValue *QualifiedIdentifier // 枚举值特判

	likely   bool // 是否使用字符串模糊查询
	optional bool // 该字段是否可选
	nullable bool // 该字段是否是一个指针类型
}

// jenConditions 生成对应的匹配条件
func (c *ListConstraint) jenConditions(field *jen.Statement, mf *FieldQualifiedIdentifier) []jen.Code {
	check := field.Clone().Op("!=")
	if c.enumValue != nil {
		check.Add(c.enumValue.AsCode()).Dot("String").Call()
	} else {
		if c.nullable {
			check.Nil()
		} else {
			check.Lit(c.skipValue)
		}
	}

	if c.nullable {
		field = jen.Op("*").Add(field)
	}

	where, binding := c.fieldName, &jen.Statement{}
	if c.likely {
		where += " like ?"
		binding = jen.Lit("%").Op("+").Add(field.Clone()).Op("+").Lit("%")
	} else {
		where += " = ?"
		if c.nullable {
			binding = jen.Func().Params().Any().Block(jen.Return(field.Clone()))
		} else {
			binding = field.Clone()
		}
	}

	return []jen.Code{check, jen.Lit(where), binding}
}

// WithDAOListerConstraint 增加一个特定类型的字段匹配
func WithDAOListerConstraint(field string, options ...ConstraintOption) DAORenderOption {
	return func(dr *daoRender) error {
		if _, ok := dr.modelFields[field]; !ok {
			return fmt.Errorf("dao: field %q not found", field)
		}

		c := &ListConstraint{fieldName: field}
		for _, opt := range options {
			opt(c)
		}

		if c.likely && c.nullable {
			return fmt.Errorf("dao: unable to mark field %q as nullable and likely both", field)
		}

		dr.listerFields = append(dr.listerFields, c)
		return nil
	}
}

// WithDAOListerConstraintLikely 增加字符串模糊查询配置
func WithDAOListerConstraintLikely(field string, optional bool) DAORenderOption {
	return WithDAOListerConstraint(field, WithConstraintOptional(optional), WithConstraintLikely())
}

// WithDAOListConstraintRequire 必选字段
func WithDAOListConstraintRequire(field string) DAORenderOption {
	return WithDAOListerConstraint(field, WithConstraintRequired())
}

// WithDAOListConstraintEnum 枚举值特判
func WithDAOListConstraintEnum(field string, skip string, optional bool) DAORenderOption {
	return WithDAOListerConstraint(field, WithConstraintAsEnumValue(skip), WithConstraintOptional(optional))
}

// ConstraintOption 配置约束的属性
type ConstraintOption func(c *ListConstraint)

// WithConstraintOptional 标记约束是可选的
func WithConstraintOptional(optional ...bool) ConstraintOption {
	return func(c *ListConstraint) {
		c.optional = len(optional) == 0 || optional[0]
	}
}

// WithConstraintRequired 表示约束是必选的
func WithConstraintRequired(required ...bool) ConstraintOption {
	return func(c *ListConstraint) {
		c.optional = !(len(required) == 0 || required[0])
	}
}

// WithConstraintNullable 表示约束是可空的
func WithConstraintNullable(nullable ...bool) ConstraintOption {
	return func(c *ListConstraint) {
		c.nullable = len(nullable) == 0 || nullable[0]
	}
}

// WithConstraintAsEnumValue 表示约束是一个 enum 值
func WithConstraintAsEnumValue(enum string) ConstraintOption {
	return func(c *ListConstraint) {
		if len(enum) != 0 {
			c.enumValue = MustParseQualifiedIdentifier(enum)
		}
	}
}

// WithConstraintLikely 标记当前约束使用模糊查询
func WithConstraintLikely(likely ...bool) ConstraintOption {
	return func(c *ListConstraint) {
		c.skipValue = ""
		c.likely = len(likely) == 0 || likely[0]
	}
}

// WithConstraintSkipCond 标记当前约束的跳过条件
func WithConstraintSkipCond(v any) ConstraintOption {
	return func(c *ListConstraint) {
		if v != nil {
			c.skipValue = v
		}
	}
}
