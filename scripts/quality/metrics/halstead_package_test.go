package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHalsteadPackageMatchesIndividualReports(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"go.mod":    "module example.test/metrics\n\ngo 1.26\n",
		"first.go":  "package example\nfunc first(a int) int { if a>0{return second(a)};return 0 }\n",
		"second.go": "package example\nfunc second(a int) int { return a+1 }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(dir, "first.go"), filepath.Join(dir, "second.go")}
	got, err := halsteadPackage(paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		want, err := halsteadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got[path], want) {
			t.Fatalf("batch differs for %s: %+v versus %+v", path, got[path], want)
		}
	}
}

func TestHalsteadPackageRejectsInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.go")
	malformed := filepath.Join(dir, "malformed.go")
	if err := os.WriteFile(valid, []byte("package example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(malformed, []byte("not Go"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][]string{nil, make([]string, 10001), {valid, valid}, {valid, filepath.Join(t.TempDir(), "other.go")}, {malformed}, {filepath.Join(dir, "missing.go")}, {dir}} {
		if _, err := halsteadPackage(paths); err == nil {
			t.Fatalf("accepted %v", paths)
		}
	}
	if err := run([]string{"halstead-package"}, io.Discard); err == nil {
		t.Fatal("accepted missing arguments")
	}
}
