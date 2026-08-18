package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
)

type Field struct {
	Name string
	Type string
	Tag  string
}

type StructInfo struct {
	Name   string
	Fields []Field
}

type ParsedFile struct {
	PackageName string
	Structs     []StructInfo
}

func ParseFile(path string) (*ParsedFile, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	result := &ParsedFile{
		PackageName: node.Name.Name,
	}

	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		info := StructInfo{Name: typeSpec.Name.Name}

		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				continue
			}

			fieldType := exprToString(field.Type)
			tag := ""
			if field.Tag != nil {
				tag = field.Tag.Value
			}

			for _, name := range field.Names {
				info.Fields = append(info.Fields, Field{
					Name: name.Name,
					Type: fieldType,
					Tag:  tag,
				})
			}
		}

		result.Structs = append(result.Structs, info)
		return true
	})

	return result, nil
}

func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	default:
		return "unknown"
	}
}
