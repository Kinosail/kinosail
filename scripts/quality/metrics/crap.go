package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func checkCRAP(path string, patterns []string, output io.Writer) error {
	profile, err := readCoverage(path)
	if err != nil {
		return err
	}
	packages, err := listPackages("", patterns, false)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return errors.New("no packages selected")
	}
	failed := false
	for _, pkg := range packages {
		bad, err := checkPackage(pkg, profile, output)
		if err != nil {
			return err
		}
		failed = failed || bad
	}
	if failed {
		return errors.New("CRAP must be less than 25")
	}
	return nil
}

func checkPackage(pkg packageInfo, profile coverageProfile, output io.Writer) (bool, error) {
	entries, err := os.ReadDir(pkg.Dir)
	if err != nil {
		return false, err
	}
	failed := false
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(pkg.Dir, name)
		info, err := entry.Info()
		if err != nil {
			return false, err
		}
		if info.ModTime().After(profile.modified) {
			return false, fmt.Errorf("coverage is older than %s; regenerate it", path)
		}
		set := token.NewFileSet()
		file, err := parseSource(set, path)
		if err != nil {
			return false, err
		}
		blocks := profile.files[pkg.ImportPath+"/"+name]
		if blocks == nil {
			blocks = profile.files[path]
		}
		bad, err := checkFunctions(set, file, blocks, output)
		if err != nil {
			return false, err
		}
		failed = failed || bad
	}
	return failed, nil
}

func checkFunctions(set *token.FileSet, file *ast.File, blocks []block, output io.Writer) (bool, error) {
	failed := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		coverage := coveredFraction(blocks, set.Position(fn.Body.Pos()).Line, set.Position(fn.Body.End()).Line)
		complexity := cyclomatic(fn)
		score := crapScore(complexity, coverage)
		if score < 25 {
			continue
		}
		failed = true
		if _, err := fmt.Fprintf(output, "%s: %s CRAP %.6f (complexity %d; covered blocks %.2f%%)\n", set.Position(fn.Pos()), fn.Name.Name, score, complexity, coverage*100); err != nil {
			return false, err
		}
	}
	return failed, nil
}

func crapScore(complexity int, coverage float64) float64 {
	missed := 1 - coverage
	c := float64(complexity)
	return c*c*missed*missed*missed + c
}

func cyclomatic(root ast.Node) int {
	count := 1
	ast.Inspect(root, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			count++
		case *ast.CaseClause:
			if n.List != nil {
				count++
			}
		case *ast.CommClause:
			if n.Comm != nil {
				count++
			}
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				count++
			}
		}
		return true
	})
	return count
}
