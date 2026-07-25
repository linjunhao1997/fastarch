package visitor

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
)

type InterfaceVisitor struct {
	Interfaces []InterfaceMeta
}

type Params []Param

type InterfaceMeta struct {
	InterfaceName string   // 接口名称
	Methods       []Method // 方法集合
}

type Method struct {
	Name    string // 方法名称
	Params  Params // 参数类型列表
	Returns Params // 返回类型
}

type Param struct {
	ParamPackage  string
	ParamName     string
	ParamType     string
	ParamBaseType string
}

type ImportInfo struct {
	Alias     string    // 导入别名（无别名时为""）
	PkgPath   string    // 完整包路径
	ImportPos token.Pos // 导入位置
}

func (v *InterfaceVisitor) Visit(n ast.Node) ast.Visitor {
	switch node := n.(type) {
	case *ast.TypeSpec:
		// 过滤Repository结尾的接口
		if iface, ok := node.Type.(*ast.InterfaceType); ok {
			meta := InterfaceMeta{
				InterfaceName: node.Name.Name,
			}

			if iface.Methods != nil {
				for _, m := range iface.Methods.List {
					switch ft := m.Type.(type) {
					case *ast.FuncType:
						method := Method{
							Name:    m.Names[0].Name,
							Params:  processParams(ft.Params),
							Returns: processParams(ft.Results),
						}
						meta.Methods = append(meta.Methods, method)
					}
				}
			}
			v.Interfaces = append(v.Interfaces, meta)
		}
	}
	return v
}

func processParams(fl *ast.FieldList) []Param {
	var params []Param
	if fl == nil {
		return params
	}

	for _, f := range fl.List {
		names := namesToStrings(f.Names)
		typeStr := types.ExprString(f.Type)
		paramPackageName := extractPkgName(f.Type)
		paramBaseType := extractBaseType(f.Type)
		// 处理多个参数名共用类型的情况（例如 a,b int）
		if len(names) == 0 { // 匿名参数
			params = append(params, Param{
				ParamPackage:  paramPackageName,
				ParamName:     "",
				ParamType:     typeStr,
				ParamBaseType: paramBaseType,
			})
		} else {
			for _, name := range names {
				params = append(params, Param{
					ParamPackage:  paramPackageName,
					ParamName:     name,
					ParamType:     typeStr,
					ParamBaseType: paramBaseType,
				})
			}
		}
	}
	return params
}

// 递归提取包名
func extractPkgName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name // 获取包别名
		}
		return extractPkgName(t.X)
	case *ast.StarExpr:
		return extractPkgName(t.X) // 处理指针类型
	case *ast.ArrayType:
		return extractPkgName(t.Elt) // 处理数组/切片
	case *ast.MapType:
		return extractPkgName(t.Value) // 取value类型包名
	default:
		return ""
	}
}

func (param Param) GetPtrStr() string {
	re := regexp.MustCompile(`^\*+`)
	return re.FindString(param.ParamType)
}

func extractBaseType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr: // 处理指针类型
		return extractBaseType(t.X)
	case *ast.ArrayType: // 处理数组/切片
		return extractBaseType(t.Elt)
	case *ast.MapType: // 处理map的value类型
		return extractBaseType(t.Value)
	case *ast.SelectorExpr: // 处理带包名的类型
		return types.ExprString(t) // 直接返回类似"model.User"
	case *ast.Ident: // 基础类型
		return t.Name
	default:
		return types.ExprString(expr)
	}
}

func namesToStrings(names []*ast.Ident) []string {
	var result []string
	for _, n := range names {
		result = append(result, n.Name)
	}
	return result
}

func (params Params) GetCtxParamName() string {
	for _, param := range params {
		if param.ParamType == "context.Context" {
			return param.ParamName
		}
	}
	return "nil"
}

func (params Params) GetBasicTypeParams() Params {
	var res Params
	for _, param := range params {
		if param.ParamType != "context.Context" {
			res = append(res, param)
		}
	}
	return res
}

func (params Params) GetOneReturn() Param {
	return params[0]
}
