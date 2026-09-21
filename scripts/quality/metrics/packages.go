package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
)

const maxSourceSize = 8 << 20

type packageInfo struct {
	Dir, ImportPath, Export string
	GoFiles, IgnoredGoFiles []string
	DepOnly                 bool
}

type boundedBuffer struct {
	bytes.Buffer
	remaining int
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if len(data) > b.remaining {
		return 0, errors.New("tool output exceeds limit")
	}
	b.remaining -= len(data)
	return b.Buffer.Write(data)
}

func listPackages(dir string, patterns []string, exports bool) ([]packageInfo, error) {
	arguments := []string{"list", "-json"}
	if exports {
		arguments = append(arguments, "-deps", "-export")
	}
	for _, pattern := range patterns {
		if pattern == "" || len(pattern) > 4096 || strings.HasPrefix(pattern, "-") || strings.ContainsAny(pattern, "\x00\r\n") {
			return nil, errors.New("invalid package pattern")
		}
	}
	arguments = append(arguments, patterns...)
	command := exec.Command("go", arguments...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	stdout, stderr := &boundedBuffer{remaining: 64 << 20}, &boundedBuffer{remaining: 1 << 20}
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("go list failed: %w: %.4096s", err, stderr.String())
	}
	decoder := json.NewDecoder(stdout)
	packages := []packageInfo{}
	for len(packages) < 10000 {
		var pkg packageInfo
		if err := decoder.Decode(&pkg); errors.Is(err, io.EOF) {
			return packages, nil
		} else if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(pkg.Dir) || pkg.ImportPath == "" {
			return nil, errors.New("invalid package metadata")
		}
		packages = append(packages, pkg)
	}
	return nil, errors.New("too many packages")
}

func validateFile(path string, limit int64) error {
	if len(path) > 4096 || strings.ContainsRune(path, 0) {
		return errors.New("invalid input path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return fmt.Errorf("input must be a regular file of at most %d bytes", limit)
	}
	return nil
}

func parseSource(set *token.FileSet, path string) (*ast.File, error) {
	if err := validateFile(path, maxSourceSize); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSourceSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSourceSize {
		return nil, errors.New("source exceeds 8 MiB")
	}
	return parser.ParseFile(set, path, data, parser.AllErrors)
}

func typedPackage(path string) (*token.FileSet, []*ast.File, *types.Info, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err := parseSource(token.NewFileSet(), path); err != nil {
		return nil, nil, nil, err
	}
	packages, err := listPackages(filepath.Dir(path), []string{"."}, true)
	if err != nil {
		return nil, nil, nil, err
	}
	set := token.NewFileSet()
	exports := map[string]string{}
	var target packageInfo
	for _, pkg := range packages {
		exports[pkg.ImportPath] = pkg.Export
		if pkg.Dir == filepath.Dir(path) {
			target = pkg
		}
	}
	files := []*ast.File{}
	var wanted *ast.File
	for _, name := range target.GoFiles {
		file, err := parseSource(set, filepath.Join(target.Dir, name))
		if err != nil {
			return nil, nil, nil, err
		}
		files = append(files, file)
		if name == filepath.Base(path) {
			wanted = file
		}
	}
	if wanted == nil {
		return nil, nil, nil, errors.New("source is not in the selected package")
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	config := types.Config{Importer: importer.ForCompiler(set, "gc", func(name string) (io.ReadCloser, error) {
		path := exports[name]
		if path == "" {
			return nil, fmt.Errorf("missing export for %s", name)
		}
		return os.Open(path)
	})}
	if _, err := config.Check(target.ImportPath, set, files, info); err != nil {
		return nil, nil, nil, err
	}
	return set, files, info, nil
}

func typedFile(path string) (*token.FileSet, *ast.File, *types.Info, error) {
	set, files, info, err := typedPackage(path)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, file := range files {
		if filepath.Base(set.Position(file.Pos()).Filename) == filepath.Base(path) {
			return set, file, info, nil
		}
	}
	return nil, nil, nil, errors.New("source is not in the selected package")
}
