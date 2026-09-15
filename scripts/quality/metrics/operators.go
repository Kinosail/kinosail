package main

import (
	"go/ast"
	"go/token"
	"reflect"
)

// These names preserve the established Go AST counting convention. Assignment
// multiplicity affects total operators, but difficulty uses distinct operators.
var nodeOperators = map[reflect.Type]string{
	reflect.TypeOf((*ast.ReturnStmt)(nil)): "return", reflect.TypeOf((*ast.IfStmt)(nil)): "if",
	reflect.TypeOf((*ast.ForStmt)(nil)): "for", reflect.TypeOf((*ast.SwitchStmt)(nil)): "switch",
	reflect.TypeOf((*ast.TypeSwitchStmt)(nil)): "type-switch", reflect.TypeOf((*ast.SelectStmt)(nil)): "select",
	reflect.TypeOf((*ast.GoStmt)(nil)): "go", reflect.TypeOf((*ast.DeferStmt)(nil)): "defer",
	reflect.TypeOf((*ast.SendStmt)(nil)): "<-", reflect.TypeOf((*ast.CallExpr)(nil)): "call",
	reflect.TypeOf((*ast.IndexExpr)(nil)): "index", reflect.TypeOf((*ast.IndexListExpr)(nil)): "index",
	reflect.TypeOf((*ast.SliceExpr)(nil)): "slice", reflect.TypeOf((*ast.SelectorExpr)(nil)): ".",
	reflect.TypeOf((*ast.CompositeLit)(nil)): "composite", reflect.TypeOf((*ast.TypeAssertExpr)(nil)): "type-assert",
	reflect.TypeOf((*ast.FuncDecl)(nil)): "func", reflect.TypeOf((*ast.FuncLit)(nil)): "func",
}

func operatorNames(node ast.Node) []string {
	if name := nodeOperators[reflect.TypeOf(node)]; name != "" {
		return []string{name}
	}
	switch n := node.(type) {
	case *ast.AssignStmt:
		return []string{n.Tok.String()}
	case *ast.BinaryExpr:
		return []string{n.Op.String()}
	case *ast.UnaryExpr:
		return []string{n.Op.String()}
	case *ast.IncDecStmt:
		return []string{n.Tok.String()}
	case *ast.BranchStmt:
		return []string{n.Tok.String()}
	case *ast.RangeStmt:
		return []string{n.Tok.String(), "range"}
	case *ast.GenDecl:
		if n.Tok == token.TYPE || n.Tok == token.VAR || n.Tok == token.CONST {
			return []string{n.Tok.String()}
		}
	case *ast.CaseClause:
		if n.List == nil {
			return []string{"default"}
		}
		return []string{"case"}
	case *ast.CommClause:
		if n.Comm == nil {
			return []string{"default"}
		}
		return []string{"case"}
	}
	return nil
}
