package settings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type libraryState struct {
	paths       []string
	editableErr error
	saveErr     error
	edits       int
	updates     int
}

func (state *libraryState) Editable() error {
	state.edits++
	return state.editableErr
}

func (state *libraryState) Read() []string { return append([]string(nil), state.paths...) }

func (state *libraryState) Update(change func([]string) ([]string, error)) error {
	next, err := change(append([]string(nil), state.paths...))
	if err != nil {
		return err
	}
	state.updates++
	if state.saveErr != nil {
		return state.saveErr
	}
	state.paths = next
	return nil
}

func TestLibraryFoldersResolveAndProjectRoots(t *testing.T) {
	t.Parallel()
	mediaRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(mediaRoot, "Movies"), 0o700); err != nil {
		t.Fatal(err)
	}
	state := &libraryState{paths: []string{".", "Movies", "Missing"}}
	roots := NewLibraryFolders(mediaRoot, state).Roots()
	resolvedRoot, err := filepath.EvalSymlinks(mediaRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 || roots[0].Path != resolvedRoot || roots[0].Namespace != "." || roots[1].Path != filepath.Join(resolvedRoot, "Movies") || roots[1].Namespace != "Movies" {
		t.Fatalf("roots = %+v", roots)
	}
	if roots := NewLibraryFolders("", state).Roots(); roots != nil {
		t.Fatalf("empty media roots = %+v", roots)
	}
}

func TestLibraryFoldersAddIsValidatedAndAtomic(t *testing.T) { //nolint:cyclop // One table proves validation and persistence have no partial effects.
	t.Parallel()
	mediaRoot := t.TempDir()
	for _, path := range []string{"Movies", "Movies/HD", "TV"} {
		if err := os.MkdirAll(filepath.Join(mediaRoot, path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	managed, failed := errors.New("managed"), errors.New("save")
	tests := []struct {
		name       string
		path       string
		configured []string
		editable   error
		save       error
		want       []string
		wantErr    string
		updates    int
	}{
		{"replace default", "Movies", []string{"."}, nil, nil, []string{"Movies"}, "", 1},
		{"append", " TV ", []string{"Movies"}, nil, nil, []string{"Movies", "TV"}, "", 1},
		{"duplicate", "Movies", []string{"Movies"}, nil, nil, []string{"Movies"}, "", 0},
		{"child overlap", "Movies/HD", []string{"Movies"}, nil, nil, []string{"Movies"}, "library folders cannot overlap", 0},
		{"parent overlap", "Movies", []string{"Movies/HD"}, nil, nil, []string{"Movies/HD"}, "library folders cannot overlap", 0},
		{"managed", "Movies", nil, managed, nil, nil, "managed", 0},
		{"save", "Movies", nil, nil, failed, nil, "save", 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &libraryState{paths: append([]string(nil), test.configured...), editableErr: test.editable, saveErr: test.save}
			err := NewLibraryFolders(mediaRoot, state).Add(test.path)
			if !reflect.DeepEqual(state.paths, test.want) || state.edits != 1 || state.updates != test.updates || test.wantErr == "" && err != nil || test.wantErr != "" && (err == nil || err.Error() != test.wantErr) {
				t.Fatalf("paths=%v edits=%d updates=%d err=%v", state.paths, state.edits, state.updates, err)
			}
		})
	}
}

func TestLibraryFoldersRemoveIsValidatedAndAtomic(t *testing.T) {
	t.Parallel()
	managed := errors.New("managed")
	tests := []struct {
		name       string
		path       string
		configured []string
		editable   error
		want       []string
		wantErr    string
		updates    int
	}{
		{"remove", " Movies ", []string{"Movies", "TV"}, nil, []string{"TV"}, "", 1},
		{"missing", "Books", []string{"Movies"}, nil, []string{"Movies"}, "library folder was not configured", 0},
		{"managed", "Movies", []string{"Movies"}, managed, []string{"Movies"}, "managed", 0},
		{"invalid", "../Movies", []string{"Movies"}, nil, []string{"Movies"}, "library folder must be inside the media mount", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &libraryState{paths: append([]string(nil), test.configured...), editableErr: test.editable}
			err := NewLibraryFolders(t.TempDir(), state).Remove(test.path)
			if !reflect.DeepEqual(state.paths, test.want) || state.edits != 1 || state.updates != test.updates || test.wantErr == "" && err != nil || test.wantErr != "" && (err == nil || err.Error() != test.wantErr) {
				t.Fatalf("paths=%v edits=%d updates=%d err=%v", state.paths, state.edits, state.updates, err)
			}
		})
	}
}

func TestLibraryFolderPathValidation(t *testing.T) {
	t.Parallel()
	mediaRoot, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(mediaRoot, "Escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaRoot, "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := []string{"", strings.Repeat("x", maximumLibraryPath+1), "bad\x00path", filepath.Join(mediaRoot, "absolute"), "..", filepath.Join("..", "outside"), "Missing", "Escape", "file"}
	for _, path := range invalid {
		state := &libraryState{}
		if err := NewLibraryFolders(mediaRoot, state).Add(path); err == nil || state.updates != 0 || len(state.paths) != 0 {
			t.Fatalf("path %q: err=%v state=%+v", path, err, state)
		}
	}
	state := &libraryState{}
	if err := NewLibraryFolders(filepath.Join(mediaRoot, "MissingRoot"), state).Add("Movies"); err == nil {
		t.Fatal("missing media root was accepted")
	}
}
