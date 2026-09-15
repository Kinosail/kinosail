// Package routeinventory discovers method-qualified HTTP routes in application
// and shared-package Go sources for security policy tests.
package routeinventory

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	maxExplicitRoutes = 2048
	maxRouteLength    = 512
)

var sharedPackages = [...]string{"federation", "downloads", "playback", "identitycore", "markers", "mediashares", "dlna", "viewing", "operations", "homeassistant", "liveevents", "watchrooms", "catalogapi", "jellyfincompat", "metadata"}

// Discover returns the unique method-qualified routes registered by an app,
// its shared route packages, and packages that expose explicit route patterns.
func Discover(appDir, packagesDir string, explicit ...[]string) ([]string, error) {
	if strings.TrimSpace(appDir) == "" || strings.TrimSpace(packagesDir) == "" {
		return nil, errors.New("app and packages directories are required")
	}
	routes := make(map[string]struct{})
	if err := addExplicit(routes, explicit); err != nil {
		return nil, err
	}
	dirs := []string{appDir}
	for _, name := range sharedPackages {
		dirs = append(dirs, filepath.Join(packagesDir, name))
	}
	for _, dir := range dirs {
		if err := scanDir(routes, dir); err != nil {
			return nil, err
		}
	}
	result := make([]string, 0, len(routes))
	for route := range routes {
		result = append(result, route)
	}
	sort.Strings(result)
	return result, nil
}

func addExplicit(routes map[string]struct{}, groups [][]string) error {
	total := 0
	for _, group := range groups {
		total += len(group)
		if total > maxExplicitRoutes {
			return fmt.Errorf("explicit route count exceeds %d", maxExplicitRoutes)
		}
		for _, route := range group {
			if !validRoute(route) {
				return fmt.Errorf("invalid explicit route %q", route)
			}
			routes[route] = struct{}{}
		}
	}
	return nil
}

func scanDir(routes map[string]struct{}, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return fmt.Errorf("scan route sources in %q: %w", dir, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("scan route sources in %q: no Go files", dir)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		if err := scanFile(routes, file); err != nil {
			return err
		}
	}
	return nil
}

func scanFile(routes map[string]struct{}, file string) error {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse route source %q: %w", file, err)
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		if route, found := registeredRoute(node); found {
			routes[route] = struct{}{}
		}
		return true
	})
	return nil
}

func registeredRoute(node ast.Node) (string, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 || !routeRegistration(call.Fun) {
		return "", false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	route, err := strconv.Unquote(literal.Value)
	return route, err == nil && validRoute(route)
}

func routeRegistration(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name == "owner" || value.Name == "handle"
	case *ast.SelectorExpr:
		return value.Sel.Name == "Handle" || value.Sel.Name == "HandleFunc"
	default:
		return false
	}
}

func validRoute(route string) bool {
	if route == "" || len(route) > maxRouteLength || strings.ContainsAny(route, "\r\n\t") {
		return false
	}
	method, path, found := strings.Cut(route, " ")
	if !found || !strings.HasPrefix(path, "/") || strings.Contains(path, " ") {
		return false
	}
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}
