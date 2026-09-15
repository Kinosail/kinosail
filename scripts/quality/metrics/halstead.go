package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

type functionMetric struct {
	Name  string `json:"name"`
	Start struct {
		Line int `json:"line"`
	} `json:"start"`
	Metrics struct {
		Difficulty float64 `json:"difficulty"`
	} `json:"metrics"`
}
type metricReport struct {
	Functions []functionMetric `json:"functions"`
}

func halsteadFile(path string) (metricReport, error) {
	set, file, info, err := typedFile(path)
	if err != nil {
		return metricReport{}, err
	}
	return measureFunctions(set, file, info), nil
}

func measureFunctions(set *token.FileSet, file *ast.File, info *types.Info) metricReport {
	report := metricReport{Functions: []functionMetric{}}
	ast.Inspect(file, func(node ast.Node) bool {
		name, literal := metricFunctionName(set, node)
		if name == "" {
			return true
		}
		value := functionMetric{Name: name}
		value.Start.Line = set.Position(node.Pos()).Line
		value.Metrics.Difficulty = halsteadDifficulty(node, info)
		report.Functions = append(report.Functions, value)
		// Existing gate counts a closure within its parent and separately, but does
		// not emit extra reports for closures nested inside that closure.
		return !literal
	})
	return report
}

func metricFunctionName(set *token.FileSet, node ast.Node) (string, bool) {
	switch fn := node.(type) {
	case *ast.FuncDecl:
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			return types.ExprString(fn.Recv.List[0].Type) + "." + fn.Name.Name, false
		}
		return fn.Name.Name, false
	case *ast.FuncLit:
		pos := set.Position(fn.Pos())
		return fmt.Sprintf("func_literal@%d:%d", pos.Line, pos.Column), true
	default:
		return "", false
	}
}

func halsteadDifficulty(node ast.Node, info *types.Info) float64 {
	operators, operands := map[string]bool{}, map[string]bool{}
	total := 0
	ast.Inspect(node, func(n ast.Node) bool {
		for _, operator := range operatorNames(n) {
			operators[operator] = true
		}
		value := operand(n, info)
		if value != "" {
			operands[value] = true
			total++
		}
		return true
	})
	if len(operands) == 0 {
		return 0
	}
	return (float64(len(operators)) / 2) * (float64(total) / float64(len(operands)))
}

func operand(node ast.Node, info *types.Info) string {
	switch n := node.(type) {
	case *ast.BasicLit:
		return n.Value
	case *ast.Ident:
		if n.Name == "_" {
			return ""
		}
		object := info.ObjectOf(n)
		if object == nil {
			return ""
		}
		prefix := operandKind(object)
		if prefix == "" {
			return ""
		}
		if pkg, ok := object.(*types.PkgName); ok {
			return prefix + pkg.Imported().Name()
		}
		if _, ok := object.(*types.Nil); ok {
			return "nil"
		}
		return prefix + object.Name()
	default:
		return ""
	}
}

func operandKind(object types.Object) string {
	switch value := object.(type) {
	case *types.Builtin:
		return "builtin:"
	case *types.Const:
		return "const:"
	case *types.Func:
		return "func:"
	case *types.Label:
		return "label:"
	case *types.Nil:
		return "nil"
	case *types.PkgName:
		return "pkg:"
	case *types.TypeName:
		return "type:"
	case *types.Var:
		if value.IsField() {
			return "field:"
		}
		return "var:"
	default:
		return ""
	}
}
