package server

import (
	"fmt"
	"io/fs"
	"os"
	"testing"
)

func fmtRemainingColdSegment(n int) string {
	return fmt.Sprintf("segment-%05d.m4s", n)
}

func cloneRemainingColdFixture(t *testing.T, source, destination string) {
	t.Helper()
	from, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer from.Close()
	if err := os.MkdirAll(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	to, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer to.Close()
	err = fs.WalkDir(from.FS(), ".", func(name string, entry fs.DirEntry, problem error) error {
		return copyRemainingColdFixtureEntry(from, to, name, entry, problem)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func copyRemainingColdFixtureEntry(from, to *os.Root, name string, entry fs.DirEntry, problem error) error {
	if problem != nil {
		return problem
	}
	if entry.Type()&os.ModeSymlink != 0 {
		return fs.ErrInvalid
	}
	if entry.IsDir() {
		return to.MkdirAll(name, 0o700)
	}
	data, err := from.ReadFile(name)
	if err != nil {
		return err
	}
	return to.WriteFile(name, data, 0o600)
}
