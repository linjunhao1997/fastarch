package gen

import (
	"fmt"
	"github.com/dave/jennifer/jen"
	"github.com/linjunhao1997/contrib/pkg/visitor"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const Repository = "Repository"
const RepositoryImpl = "RepositoryImpl"

const MethodQuery = "Query"
const MethodInsert = "Insert"
const MethodDelete = "Delete"
const MethodUpdate = "Update"

//var packageModelDict = map[string]visitor.StructMeta{}

type ModelColumn struct {
	ModelName  string
	FieldName  string
	ColumnName string
}

func (c *ModelColumn) GetColumnConstant() string {
	return fmt.Sprintf("%sColumn%s", c.ModelName, c.FieldName)
}

func GenerateGormMysqlRepository(interfaceFilePath string, modelFilePath string) error {
	interfaceFilePathInfo := ParseFilePath(interfaceFilePath)

	modelFilePathInfo := ParseFilePath(modelFilePath)
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, modelFilePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	s := &visitor.StructVisitor{}
	ast.Walk(s, file)

	modelPackageName := file.Name.Name

	f := jen.NewFile(modelPackageName)
	for _, e := range s.Structs {
		//packageModelDict[file.Name.Name+"."+e.StructName] = e
		var modelColumns []ModelColumn
		for _, field := range e.Fields {
			value, ok := field.Tag.Lookup("gorm")
			if !ok {
				continue
			}
			properties := strings.Split(value, ",")
			for _, property := range properties {
				kv := strings.Split(property, ":")
				if len(kv) != 2 {
					continue
				}
				k := kv[0]
				if k != "column" {
					continue
				}
				columnName := strings.Split(kv[1], ";")[0]
				modelColumns = append(modelColumns, ModelColumn{
					ModelName:  e.StructName,
					FieldName:  field.Name,
					ColumnName: columnName,
				})
				break
			}
		}

		curTime := time.Now().Format("2006-01-02 15:04:05")

		f.PackageComment("// Do not Edit Generated File.")
		f.PackageComment(fmt.Sprintf("// Generated at: %s\n", curTime))
		list := make([]jen.Code, 0)
		for _, e := range modelColumns {
			list = append(list, jen.Id(e.GetColumnConstant()).Op("=").Lit(e.ColumnName))
		}
		f.Const().Defs(list...)

	}
	if err = f.Save(fmt.Sprintf("%s/%s_generated.go", modelFilePathInfo.Dir, modelFilePathInfo.FileName)); err != nil {
		return err
	}

	if len(interfaceFilePath) == 0 {
		return nil
	}
	file, err = parser.ParseFile(fset, interfaceFilePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	v := &visitor.InterfaceVisitor{}
	ast.Walk(v, file)

	repositoryInterfaces := make([]visitor.InterfaceMeta, 0)

	f = jen.NewFile("repository")
	imports, err := GetFileImports(file)
	if err != nil {
		return err
	}
	for _, e := range imports {
		importNames[e.Alias] = e.PkgPath
		f.ImportName(e.PkgPath, e.Alias)
	}

	for _, e := range v.Interfaces {
		if !strings.HasSuffix(e.InterfaceName, Repository) {
			continue
		}
		repositoryInterfaces = append(repositoryInterfaces, e)
	}

	// 生成代码接口代码
	for _, repo := range repositoryInterfaces {
		modelName := strings.TrimSuffix(repo.InterfaceName, Repository)
		// 生成结构体

		f.Type().Id(modelName + RepositoryImpl).Struct(
			jen.Id("DB").Op("*").Qual(importNames["gorm"], "DB"),
		)

		// 生成构造函数
		f.Func().Id("New" + modelName + Repository).Params(jen.Id("db").Op("*").Qual(importNames["gorm"], "DB")).Op("*").Id(modelName + RepositoryImpl).Block(
			jen.Return(jen.Op("&").Id(modelName + RepositoryImpl).Values(jen.Id("DB").Op(":").Id("db"))),
		)

		for _, method := range repo.Methods {
			f.Line()
			f.Func().Params(jen.Id("r").Op("*").Id(modelName + RepositoryImpl)).Id(method.Name).
				Params(parseParams(method.Params)...).
				Params(parseParams(method.Returns)...).
				BlockFunc(func(g *jen.Group) {
					switch {
					case strings.HasPrefix(method.Name, MethodQuery):
						conditionFields := extractFields(method.Name)

						// 生成标准查询逻辑
						var data = "data"
						g.Var().Id(data).Id(method.Returns.GetOneReturn().ParamBaseType)
						g.If(
							jen.Id("err").Op(":=").Id("r").Dot("DB").Do(WhereConditionStatement(conditionFields, method.Params, method.Returns)).
								Dot("First").Call(jen.Op("&").Id(data)).Dot("Error"),

							jen.Id("err").Op("!=").Nil(),
						).Block(
							jen.Return(jen.Nil(), jen.Id("err")),
						)

						g.Return(jen.Op("&").Id(data), jen.Nil())

					case strings.HasPrefix(method.Name, "Create"):

						// 生成带错误处理的插入逻辑
						g.If(
							jen.Id("err").Op(":=").Id("r").Dot("DB").
								Do(CreateStatement(method.Params)).
								Dot("Error"),
							jen.Id("err").Op("!=").Nil(),
						).Block(
							jen.Return(jen.Id("err")),
						)
						// 最后返回nil
						g.Return(jen.Nil())

					default:
						// 生成默认错误返回
						g.Return(jen.Nil(),
							jen.Qual("errors", "New").
								Call(jen.Lit("未实现方法: "+modelName)),
						)
					}
				})
		}
		if err = f.Save(fmt.Sprintf("%s/%s_generated.go", interfaceFilePathInfo.Dir, interfaceFilePathInfo.FileName)); err != nil {
			return err
		}
	}
	return nil
}

// 参数类型转换
func parseParams(params []visitor.Param) []jen.Code {

	var jenParams []jen.Code
	for _, p := range params {
		el := strings.Split(p.ParamBaseType, ".")
		if len(el) == 1 {
			jenParams = append(jenParams, jen.Id(p.ParamName).Id(p.ParamType))
		} else if len(el) > 1 {
			jenParams = append(jenParams, jen.Id(p.ParamName).Op(p.GetPtrStr()).Qual(importNames[p.ParamPackage], el[1]))
		}
	}
	return jenParams
}

func WhereConditionStatement(conditionFields []string, params visitor.Params, returns visitor.Params) func(statement *jen.Statement) {
	returnParam := returns.GetOneReturn()
	//key := returnParam.ParamBaseType
	//modelName := packageModelDict[key].StructName
	return func(statement *jen.Statement) {
		statement.Dot("WithContext").Call(jen.Id(params.GetCtxParamName()))
		for i, param := range params.GetBasicTypeParams() {
			statement.Dot("Where").Call(jen.Id(returnParam.ParamBaseType+"Column"+conditionFields[i]), jen.Id(param.ParamName))
		}
	}
}

func CreateStatement(params visitor.Params) func(statement *jen.Statement) {
	return func(statement *jen.Statement) {
		statement.Dot("WithContext").Call(jen.Id(params.GetCtxParamName()))
		for _, param := range params.GetBasicTypeParams() {
			statement.Dot("Create").Call(jen.Id(param.ParamName))
		}
	}
}

func CamelToSnake(s string) string {
	var builder strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			// 前一个字符不是下划线时添加分隔符
			if prev := s[i-1]; prev != '_' && !unicode.IsUpper(rune(prev)) {
				builder.WriteByte('_')
			}
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}

// 路径解析结果结构体
type PathInfo struct {
	FullPath string // 完整路径
	Dir      string // 目录路径
	FileName string // 文件名(不含扩展名)
	Ext      string // 文件扩展名(包含点)
}

func ParseFilePath(fullPath string) PathInfo {
	// 获取目录路径
	dir := filepath.Dir(fullPath)
	// 获取带扩展的文件名
	base := filepath.Base(fullPath)
	// 分离扩展名
	ext := filepath.Ext(base)
	// 去除扩展名的文件名
	name := strings.TrimSuffix(base, ext)

	return PathInfo{
		FullPath: fullPath,
		Dir:      dir,
		FileName: name,
		Ext:      ext,
	}
}

type ImportInfo struct {
	Alias     string    // 导入别名（无别名时为""）
	PkgPath   string    // 完整包路径
	ImportPos token.Pos // 导入位置
}

// 获取当前文件所有导入的包
func GetFileImports(node *ast.File) ([]ImportInfo, error) {
	var imports []ImportInfo
	for _, imp := range node.Imports {
		info := ImportInfo{
			PkgPath:   strings.Trim(imp.Path.Value, `"`),
			ImportPos: imp.Pos(),
		}

		// 处理导入语法树结构
		switch {
		case imp.Name != nil:
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				// 从包路径提取最后一段作为默认包名
				pathParts := strings.Split(info.PkgPath, "/")
				info.Alias = pathParts[len(pathParts)-1]
			} else {
				info.Alias = imp.Name.Name // 普通别名
			}
		default:
			// 从包路径提取最后一段作为默认包名
			pathParts := strings.Split(info.PkgPath, "/")
			info.Alias = pathParts[len(pathParts)-1]
		}

		imports = append(imports, info)
	}
	return imports, nil
}

var importNames = map[string]string{}

func extractFields(methodName string) []string {
	re := regexp.MustCompile(`[A-Z][a-z]*`)
	words := re.FindAllString(methodName, -1)
	connectors := map[string]bool{"And": true, "Or": true, "With": true}

	var fields []string
	foundBy := false

	for _, word := range words {
		if word == "By" {
			foundBy = true
			continue
		}
		if foundBy && !connectors[word] {
			fields = append(fields, word)
		}
	}
	return fields
}
