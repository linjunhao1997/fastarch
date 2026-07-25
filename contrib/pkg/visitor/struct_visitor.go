package visitor

import (
	"go/ast"
	"go/token"
	"reflect"
	"strings"
)

// 结构体信息收集器
type StructVisitor struct {
	Structs []StructMeta // 存储解析结果
}

// 结构体元数据
type StructMeta struct {
	StructName string      // 结构体名称
	Fields     []FieldMeta // 字段集合
	Pos        token.Pos   // 代码位置
}

// 字段元数据
type FieldMeta struct {
	Name string            // 字段名称
	Type string            // 字段类型
	Tag  reflect.StructTag // 结构体标签
}

// Visitor接口实现
func (v *StructVisitor) Visit(n ast.Node) ast.Visitor {
	switch node := n.(type) {
	case *ast.TypeSpec: // 类型定义节点
		if structType, ok := node.Type.(*ast.StructType); ok {
			meta := StructMeta{
				StructName: node.Name.Name,
				Pos:        node.Pos(),
			}

			// 提取字段信息
			if structType.Fields != nil {
				for _, field := range structType.Fields.List {
					// 获取字段类型表达式
					typeExpr := stringifyExpr(field.Type)

					// 处理多个字段名称 (e.g. x,y int)
					for _, name := range field.Names {
						f := FieldMeta{
							Name: name.Name,
							Type: typeExpr,
						}

						// 提取结构体标签
						if field.Tag != nil {
							f.Tag = reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
						}

						meta.Fields = append(meta.Fields, f)
					}

					// 处理匿名嵌入字段
					if len(field.Names) == 0 {
						f := FieldMeta{
							Type: typeExpr,
						}
						meta.Fields = append(meta.Fields, f)
					}
				}
			}

			v.Structs = append(v.Structs, meta)
		}
	}
	return v // 持续遍历
}

// 递归解析类型表达式
func stringifyExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident: // 基础类型
		return t.Name
	case *ast.StarExpr: // 指针类型
		return "*" + stringifyExpr(t.X)
	case *ast.ArrayType: // 数组/slice
		if t.Len == nil {
			return "[]" + stringifyExpr(t.Elt)
		}
		return "[" + stringifyExpr(t.Len) + "]" + stringifyExpr(t.Elt)
	case *ast.SelectorExpr: // 外部类型 (e.g. time.Time)
		return stringifyExpr(t.X) + "." + t.Sel.Name
	case *ast.MapType: // 字典类型
		return "map[" + stringifyExpr(t.Key) + "]" + stringifyExpr(t.Value)
	case *ast.InterfaceType: // 接口类型
		return "interface{}"
	default:
		return "unknown_type"
	}
}
