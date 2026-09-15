package settings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/catalog"
)

const maximumLibraryPath = 4096

var errUnchangedLibraryFolders = errors.New("library folders are unchanged")

// LibraryFolderState adapts app-owned settings persistence to folder operations.
// Read returns a detached snapshot. Update validates, saves, and commits under one exclusive lock.
// Update must leave active state unchanged when validation or persistence fails.
type LibraryFolderState interface {
	Editable() error
	Read() []string
	Update(func([]string) ([]string, error)) error
}

// LibraryFolders owns canonical validation and configured-folder changes.
type LibraryFolders struct {
	mediaRoot string
	state     LibraryFolderState
}

// NewLibraryFolders binds canonical folder rules to app-owned settings state.
func NewLibraryFolders(mediaRoot string, state LibraryFolderState) LibraryFolders {
	return LibraryFolders{mediaRoot, state}
}

// Roots resolves valid configured folders for one Library scan.
func (folders LibraryFolders) Roots() []catalog.ScanRoot {
	if folders.mediaRoot == "" {
		return nil
	}
	configured := folders.state.Read()
	roots := make([]catalog.ScanRoot, 0, len(configured))
	for _, relative := range configured {
		if root, err := folders.resolve(relative); err == nil {
			roots = append(roots, root)
		}
	}
	return roots
}

// Add validates and persists one folder inside the media mount.
func (folders LibraryFolders) Add(path string) error {
	if err := folders.state.Editable(); err != nil {
		return err
	}
	root, err := folders.resolve(path)
	if err != nil {
		return err
	}
	err = folders.state.Update(func(configured []string) ([]string, error) {
		if len(configured) == 1 && configured[0] == "." {
			configured = nil
		}
		for _, existing := range configured {
			if existing == root.Namespace {
				return nil, errUnchangedLibraryFolders
			}
			if within(existing, root.Namespace) || within(root.Namespace, existing) {
				return nil, errors.New("library folders cannot overlap")
			}
		}
		return append(configured, root.Namespace), nil
	})
	if errors.Is(err, errUnchangedLibraryFolders) {
		return nil
	}
	return err
}

// Remove validates and persists removal of one configured folder.
func (folders LibraryFolders) Remove(path string) error {
	if err := folders.state.Editable(); err != nil {
		return err
	}
	path, err := normalizeLibraryPath(path)
	if err != nil {
		return err
	}
	path = filepath.ToSlash(path)
	return folders.state.Update(func(configured []string) ([]string, error) {
		remaining := make([]string, 0, len(configured))
		found := false
		for _, existing := range configured {
			if existing == path {
				found = true
				continue
			}
			remaining = append(remaining, existing)
		}
		if !found {
			return nil, errors.New("library folder was not configured")
		}
		return remaining, nil
	})
}

func (folders LibraryFolders) resolve(path string) (catalog.ScanRoot, error) {
	relative, err := normalizeLibraryPath(path)
	if err != nil {
		return catalog.ScanRoot{}, err
	}
	root, rootErr := filepath.EvalSymlinks(folders.mediaRoot)
	full, fullErr := filepath.EvalSymlinks(filepath.Join(root, relative))
	inside, relErr := filepath.Rel(root, full)
	if rootErr != nil || fullErr != nil || relErr != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return catalog.ScanRoot{}, errors.New("library folder must be an existing folder inside the media mount")
	}
	info, err := os.Stat(full)
	if err != nil || !info.IsDir() {
		return catalog.ScanRoot{}, errors.New("library folder must be an existing folder inside the media mount")
	}
	return catalog.ScanRoot{Path: full, Namespace: filepath.ToSlash(relative)}, nil
}

func normalizeLibraryPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || len(path) > maximumLibraryPath || strings.ContainsRune(path, 0) || filepath.IsAbs(path) {
		return "", errors.New("library folder must be inside the media mount")
	}
	path = filepath.Clean(path)
	if path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", errors.New("library folder must be inside the media mount")
	}
	return path, nil
}

func within(parent, child string) bool {
	return parent == "." || child == parent || strings.HasPrefix(child, parent+"/")
}
