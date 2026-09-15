package routeinventory

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

func TestInventoryRemainingParserAndCardinalityEdges(t *testing.T) {
	t.Parallel()
	routes := make(map[string]struct{})
	if err := addExplicit(routes, [][]string{make([]string, maxExplicitRoutes+1)}); err == nil {
		t.Fatal("oversized explicit route inventory was accepted")
	}
	if err := scanDir(routes, "["); err == nil {
		t.Fatal("malformed source glob was accepted")
	}

	for name, node := range map[string]ast.Node{
		"non call":       &ast.Ident{Name: "route"},
		"missing args":   &ast.CallExpr{Fun: &ast.Ident{Name: "handle"}},
		"unknown helper": &ast.CallExpr{Fun: &ast.Ident{Name: "other"}, Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `"GET /"`}}},
		"non literal":    &ast.CallExpr{Fun: &ast.Ident{Name: "handle"}, Args: []ast.Expr{&ast.Ident{Name: "route"}}},
		"non string":     &ast.CallExpr{Fun: &ast.Ident{Name: "handle"}, Args: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}}},
	} {
		t.Run(name, func(t *testing.T) {
			if route, found := registeredRoute(node); found || route != "" {
				t.Fatalf("registered route = %q, %v", route, found)
			}
		})
	}
	if routeRegistration(&ast.BasicLit{Kind: token.STRING, Value: `"handle"`}) {
		t.Fatal("non-function route registration was accepted")
	}
	if validRoute("GET /" + strings.Repeat("a", maxRouteLength)) {
		t.Fatal("oversized route was accepted")
	}
}
