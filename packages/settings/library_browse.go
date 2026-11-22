package settings

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const (
	maximumBrowseEntries = 4000
	maximumBrowseFolders = 300
	maximumEstimateScan  = 20000
	maximumEstimateDepth = 4
)

var systemFolders = map[string]bool{
	"bin": true, "boot": true, "dev": true, "etc": true, "lib": true, "lib32": true, "lib64": true, "libx32": true,
	"lost+found": true, "opt": true, "proc": true, "root": true, "run": true, "sbin": true, "snap": true, "sys": true,
	"tmp": true, "usr": true, "var": true, "System": true, "Library": true, "Applications": true, "private": true,
	"cores": true, "Windows": true, "Program Files": true, "Program Files (x86)": true, "ProgramData": true,
	"$Recycle.Bin": true, "System Volume Information": true, "Recovery": true,
}

var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".wmv": true, ".ts": true, ".m2ts": true,
	".webm": true, ".mpg": true, ".mpeg": true, ".flv": true, ".vob": true, ".iso": true,
}

type FolderListing struct {
	Path      string
	Parent    string
	Folders   []string
	Truncated bool
}

type VideoEstimate struct {
	Count    int
	Complete bool
}

func (folders LibraryFolders) Browse(relative string) (FolderListing, error) {
	if relative == "" {
		relative = "."
	}
	root, err := folders.resolve(relative)
	if err != nil {
		return FolderListing{}, err
	}
	directory, err := os.Open(root.Path)
	if err != nil {
		return FolderListing{}, errors.New("folder cannot be opened")
	}
	defer directory.Close()
	entries, err := directory.ReadDir(maximumBrowseEntries)
	if err != nil && !errors.Is(err, io.EOF) {
		return FolderListing{}, errors.New("folder cannot be read")
	}
	listing := FolderListing{Path: root.Namespace, Truncated: len(entries) == maximumBrowseEntries}
	if root.Namespace != "." {
		listing.Parent = path.Dir(root.Namespace)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !browsableFolder(root.Namespace, name, entry) {
			continue
		}
		listing.Folders = append(listing.Folders, name)
	}
	slices.SortFunc(listing.Folders, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	if len(listing.Folders) > maximumBrowseFolders {
		listing.Folders, listing.Truncated = listing.Folders[:maximumBrowseFolders], true
	}
	return listing, nil
}

func browsableFolder(parent, name string, entry fs.DirEntry) bool {
	if strings.HasPrefix(name, ".") || strings.ContainsAny(name, "\x00\n\r") || parent == "." && systemFolders[name] {
		return false
	}
	if entry.IsDir() {
		return true
	}
	return entry.Type()&fs.ModeSymlink != 0
}

func (folders LibraryFolders) Shortcuts(patterns []string) []string {
	root, err := filepath.EvalSymlinks(folders.mediaRoot)
	if err != nil {
		return nil
	}
	var found []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			continue
		}
		for _, match := range matches {
			relative, err := filepath.Rel(root, match)
			if err != nil || strings.Contains(relative, string(filepath.Separator)+".") || strings.HasPrefix(relative, ".") {
				continue
			}
			if resolved, err := folders.resolve(relative); err == nil && resolved.Namespace != "." && !slices.Contains(found, resolved.Namespace) {
				found = append(found, resolved.Namespace)
			}
			if len(found) == 12 {
				return found
			}
		}
	}
	return found
}

func (folders LibraryFolders) EstimateVideos(relative string) (VideoEstimate, error) {
	root, err := folders.resolve(relative)
	if err != nil {
		return VideoEstimate{}, err
	}
	estimate, scanned := VideoEstimate{Complete: true}, 0
	walkErr := filepath.WalkDir(root.Path, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		scanned++
		if scanned > maximumEstimateScan {
			estimate.Complete = false
			return filepath.SkipAll
		}
		if entry.IsDir() {
			if current != root.Path && (strings.HasPrefix(entry.Name(), ".") || strings.Count(strings.TrimPrefix(current, root.Path), string(filepath.Separator)) > maximumEstimateDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		if videoExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			estimate.Count++
		}
		return nil
	})
	if walkErr != nil {
		return VideoEstimate{}, errors.New("folder cannot be read")
	}
	return estimate, nil
}
