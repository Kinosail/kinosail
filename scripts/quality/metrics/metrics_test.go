package main

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, source string) (*token.FileSet, *ast.File, *types.Info) {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "sample.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	config := types.Config{Importer: importer.Default()}
	if _, err := config.Check("sample", set, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	return set, file, info
}

func TestHalsteadCountsTypedOperandsAndNestedFunctions(t *testing.T) {
	set, file, info := fixture(t, `package sample
 func add(a,b int)int{return a+b}
 func nested(x int)int { f:=func()int{return x}; return f() }
 `)
	got := measureFunctions(set, file, info)
	if len(got.Functions) != 3 {
		t.Fatalf("reports: %+v", got)
	}
	// func,+,return; func:add,var:a,var:b,type:int; seven operand occurrences.
	if got.Functions[0].Metrics.Difficulty != 2.625 {
		t.Fatalf("add: %+v", got.Functions[0])
	}
	if !strings.HasPrefix(got.Functions[2].Name, "func_literal@") {
		t.Fatalf("closure: %+v", got)
	}
}

func TestCRAPBoundaryAndBranches(t *testing.T) {
	_, file, _ := fixture(t, `package sample
 func f(x bool){ if x && !x {} else if x || !x {} ; for x {}; for range []int{} {}; switch {case x: default:}; select{default:} }
 `)
	if got := cyclomatic(file.Decls[0]); got != 8 {
		t.Fatalf("complexity: %d", got)
	}
	if crapScore(25, 1) != 25 || crapScore(24, 1) != 24 || crapScore(5, 0) != 30 {
		t.Fatal("CRAP boundary changed")
	}
	if got := coveredFraction([]block{{1, 2, false}, {1, 2, true}, {3, 4, false}, {8, 9, true}}, 1, 4); got != .5 {
		t.Fatalf("duplicate blocks: %g", got)
	}
	if coveredFraction(nil, 1, 2) != 0 {
		t.Fatal("missing coverage was optimistic")
	}
}

func TestCoverageRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "mode: mystery\na:1.1,2.1 1 1\n", "mode: set\n", "mode: set\ngarbage\n", "mode: set\na:0.1,2.1 1 1\n", "mode: set\na:3.1,2.1 1 1\n", "mode: set\na:1.2,1.1 1 1\n", "mode: set\na:1.1,2.1 1 -1\n", "mode: set\na:1.1,2.1 1 99999999999999999999\n", "mode: set\n" + strings.Repeat("x", 9000)} {
		if _, err := parseCoverage(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %.60q", input)
		}
	}
	for _, mode := range []string{"set", "count", "atomic"} {
		if _, err := parseCoverage(strings.NewReader("mode: " + mode + "\nsample/a.go:1.1,2.1 1 1\n")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGateRejectsStaleProfileAndKeepsMethodCoverageSeparate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	source := "package sample\ntype A int\ntype B int\nfunc (A) F(x bool) {if x{};if x{};if x{};if x{}}\nfunc (B) F(x bool) {if x{};if x{};if x{};if x{}}\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := packageInfo{Dir: dir, ImportPath: "sample"}
	profile := coverageProfile{modified: time.Now().Add(-time.Hour), files: map[string][]block{"sample/sample.go": {{4, 4, true}, {5, 5, false}}}}
	if _, err := checkPackage(pkg, profile, io.Discard); err == nil {
		t.Fatal("accepted stale coverage")
	}
	profile.modified = time.Now().Add(time.Hour)
	var output bytes.Buffer
	failed, err := checkPackage(pkg, profile, &output)
	if err != nil || !failed || !strings.Contains(output.String(), "sample.go:5:") || strings.Contains(output.String(), "sample.go:4:") {
		t.Fatalf("%v %v %s", failed, err, output.String())
	}
}

func TestCommandsValidateBeforeExecutingTools(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"crap"}, {"halstead"}, {"halstead", "a", "b"}, {"crap", "missing", "."}} {
		if err := run(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if _, err := listPackages("", []string{"-test"}, false); err == nil {
		t.Fatal("accepted option injection")
	}
	b := boundedBuffer{remaining: 2}
	if _, err := b.Write([]byte("abc")); err == nil || b.Len() != 0 {
		t.Fatal("output bound failed")
	}
	if _, err := b.Write([]byte("ab")); err != nil {
		t.Fatal(err)
	}
}

func TestSourceRejectsInvalidAndOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	large := filepath.Join(dir, "large.go")
	if err := os.WriteFile(large, make([]byte, maxSourceSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, filepath.Join(dir, "missing.go"), "bad\x00path", strings.Repeat("x", 4097), large} {
		if _, err := parseSource(token.NewFileSet(), path); err == nil {
			t.Fatalf("accepted source %.80q", path)
		}
	}
}

func TestHalsteadRejectsSourceBeforeRunningTools(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	path := filepath.Join(dir, "large.go")
	if err := os.WriteFile(path, make([]byte, maxSourceSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"halstead", path}, io.Discard); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("expected source validation before go invocation: %v", err)
	}
}

func TestCoverageRejectsFIFOWithoutBlocking(t *testing.T) {
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo unavailable")
	}
	path := filepath.Join(t.TempDir(), "profile")
	if err := exec.Command(mkfifo, path).Run(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := readCoverage(path)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("accepted FIFO")
		}
	case <-time.After(time.Second):
		t.Fatal("coverage input blocked before regular-file validation")
	}
}
