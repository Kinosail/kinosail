package routeinventory

import (
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoverIncludesAppSharedAndExplicitRoutes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	packagesDir := filepath.Join(root, "packages")
	writeRouteSource(t, appDir, "routes.go", `package app
func register(mux interface{ HandleFunc(string, any) }, owner func(string, any)) {
	mux.HandleFunc("GET /app", nil)
	owner("POST /owner", nil)
	// mux.HandleFunc("GET /commented", nil)
}`)
	writeRouteSource(t, appDir, "ignored_test.go", `package app
func ignored(mux interface{ HandleFunc(string, any) }) { mux.HandleFunc("GET /test", nil) }`)
	for _, name := range sharedPackages {
		writeRouteSource(t, filepath.Join(packagesDir, name), name+".go", "package "+name+"\n")
	}
	writeRouteSource(t, filepath.Join(packagesDir, "downloads"), "routes.go", `package downloads
func register(mux interface{ Handle(string, any) }) { mux.Handle("DELETE /shared/{id}", nil) }`)
	writeRouteSource(t, filepath.Join(packagesDir, "metadata"), "routes.go", `package metadata
func register(mux interface{ Handle(string, any) }) { mux.Handle("GET /metadata/bulk", nil) }`)
	writeRouteSource(t, filepath.Join(packagesDir, "viewing"), "routes.go", `package viewing
func register(handle func(string, any)) { handle("PUT /helper/{id}", nil) }`)

	routes, err := Discover(appDir, packagesDir, []string{"HEAD /explicit", "GET /app"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"DELETE /shared/{id}", "GET /app", "GET /metadata/bulk", "HEAD /explicit", "POST /owner", "PUT /helper/{id}"}
	if !reflect.DeepEqual(routes, want) {
		t.Fatalf("routes = %q, want %q", routes, want)
	}
}

func TestDiscoverRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	packagesDir := filepath.Join(root, "packages")
	writeRouteSource(t, appDir, "routes.go", "package app\n")
	for _, name := range sharedPackages {
		writeRouteSource(t, filepath.Join(packagesDir, name), name+".go", "package "+name+"\n")
	}

	tests := []struct {
		name, appDir, packagesDir string
		explicit                  [][]string
	}{
		{name: "missing app", packagesDir: packagesDir},
		{name: "missing packages", appDir: appDir},
		{name: "unknown method", appDir: appDir, packagesDir: packagesDir, explicit: [][]string{{"TRACE /debug"}}},
		{name: "missing path", appDir: appDir, packagesDir: packagesDir, explicit: [][]string{{"GET"}}},
		{name: "oversized route", appDir: appDir, packagesDir: packagesDir, explicit: [][]string{{"GET /" + strings.Repeat("a", maxRouteLength)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			routes, err := Discover(test.appDir, test.packagesDir, test.explicit...)
			if err == nil || routes != nil {
				t.Fatalf("Discover() = %q, %v; want nil error result", routes, err)
			}
		})
	}
}

func TestRouteDiscoveryCoversRejectedSourceShapes(t *testing.T) {
	t.Parallel()
	routes := make(map[string]struct{})
	if err := addExplicit(routes, [][]string{make([]string, maxExplicitRoutes+1)}); err == nil {
		t.Fatal("oversized explicit route group accepted")
	}
	if err := scanDir(routes, "["); err == nil {
		t.Fatal("malformed glob path accepted")
	}

	expressions := []ast.Expr{
		&ast.FuncLit{},
		&ast.CallExpr{Fun: ast.NewIdent("handle"), Args: []ast.Expr{ast.NewIdent("pattern")}},
	}
	for _, expression := range expressions {
		if _, found := registeredRoute(expression); found {
			t.Fatalf("registeredRoute(%T) found a route", expression)
		}
	}
	if routeRegistration(&ast.CallExpr{}) {
		t.Fatal("call expression accepted as route registration")
	}
}

func TestDiscoverReportsMissingAndMalformedSources(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	packagesDir := filepath.Join(root, "packages")
	writeRouteSource(t, appDir, "routes.go", "package app\n")
	for _, name := range sharedPackages {
		writeRouteSource(t, filepath.Join(packagesDir, name), name+".go", "package "+name+"\n")
	}

	if err := os.Remove(filepath.Join(packagesDir, "playback", "playback.go")); err != nil {
		t.Fatal(err)
	}
	if routes, err := Discover(appDir, packagesDir); err == nil || routes != nil {
		t.Fatalf("missing source directory = %q, %v; want error", routes, err)
	}
	writeRouteSource(t, filepath.Join(packagesDir, "playback"), "playback.go", "package playback\nfunc broken(")
	if routes, err := Discover(appDir, packagesDir); err == nil || routes != nil {
		t.Fatalf("malformed source = %q, %v; want error", routes, err)
	}
}

func writeRouteSource(t *testing.T, dir, name, source string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}
