package library

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	archiveEntryLimit = 10000
	archiveListLimit  = 4 << 20
	archiveAssetLimit = 64 << 20
)

// ArchiveImage identifies one safe image entry inside a comic archive.
type ArchiveImage struct{ Name, Title string }

// ArchiveImages returns the bounded, sorted image entries in one library item.
func ArchiveImages(ctx context.Context, item Item) []ArchiveImage {
	images := make([]ArchiveImage, 0)
	for _, name := range archiveEntries(ctx, item) {
		name = CleanArchivePath(name)
		if name == "" {
			continue
		}
		switch strings.ToLower(filepath.Ext(name)) {
		case ".avif", ".gif", ".jpeg", ".jpg", ".png", ".webp":
			images = append(images, ArchiveImage{Name: name, Title: filepath.Base(name)})
		}
	}
	sort.Slice(images, func(left, right int) bool {
		return strings.ToLower(images[left].Title) < strings.ToLower(images[right].Title)
	})
	return images
}

func archiveEntries(ctx context.Context, item Item) []string {
	if strings.EqualFold(item.Container, "CBZ") {
		archive, err := zip.OpenReader(item.Path)
		if err != nil {
			return nil
		}
		defer archive.Close()
		entries := make([]string, 0, min(len(archive.File), archiveEntryLimit))
		for _, file := range archive.File[:min(len(archive.File), archiveEntryLimit)] {
			entries = append(entries, file.Name)
		}
		return uniqueArchiveEntries(entries)
	}
	data, err := archiveCommand(ctx, archiveListLimit, "-tf", item.Path)
	if err != nil {
		return nil
	}
	entries := strings.Split(strings.TrimSpace(string(data)), "\n")
	return uniqueArchiveEntries(entries)
}

func uniqueArchiveEntries(entries []string) []string {
	seen := make(map[string]struct{}, min(len(entries), archiveEntryLimit))
	for index, entry := range entries {
		name := CleanArchivePath(entry)
		if name == "" {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			return nil
		}
		if index < archiveEntryLimit {
			seen[name] = struct{}{}
		}
	}
	return entries[:min(len(entries), archiveEntryLimit)]
}

// ReadArchiveAsset returns one validated and bounded entry from a reader archive.
func ReadArchiveAsset(ctx context.Context, item Item, name string) ([]byte, error) {
	name = CleanArchivePath(name)
	if name == "" {
		return nil, errors.New("archive entry not found")
	}
	if strings.EqualFold(item.Container, "CB7") || strings.EqualFold(item.Container, "CBT") {
		found := false
		for _, entry := range archiveEntries(ctx, item) {
			if CleanArchivePath(entry) == name {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("archive entry not found")
		}
		pattern := strings.NewReplacer("*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]", "^", "\\^", "$", "\\$").Replace(name)
		return archiveCommand(ctx, archiveAssetLimit, "-xnOf", item.Path, "--", pattern)
	}
	archive, err := zip.OpenReader(item.Path)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	return ReadArchive(archive.File, name, archiveAssetLimit)
}

func archiveCommand(ctx context.Context, limit int64, arguments ...string) ([]byte, error) {
	return boundedCommand(ctx, "bsdtar", limit, arguments...)
}

func boundedCommand(parent context.Context, executable string, limit int64, arguments ...string) ([]byte, error) {
	if parent == nil || executable == "" || limit <= 0 {
		return nil, errors.New("archive reader unavailable")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	//nolint:gosec // Arguments are fixed flags, a scanned library path, and a validated archive entry.
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Stderr = io.Discard
	output, err := command.StdoutPipe()
	if err != nil || command.Start() != nil {
		return nil, errors.New("archive reader unavailable")
	}
	data, readErr := boundedOutput(output, limit)
	if readErr != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, readErr
	}
	if err := command.Wait(); err != nil {
		return nil, errors.New("archive could not be read")
	}
	return data, nil
}

func boundedOutput(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("archive output is invalid")
	}
	return data, nil
}

// CleanArchivePath accepts one relative archive path without traversal segments.
func CleanArchivePath(name string) string {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return ""
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return ""
		}
	}
	return path.Clean(name)
}

// ReadArchive returns one bounded entry from an open ZIP-compatible archive.
func ReadArchive(files []*zip.File, name string, limit int64) ([]byte, error) {
	name = CleanArchivePath(name)
	if name == "" || limit <= 0 {
		return nil, errors.New("archive entry not found")
	}
	files = files[:min(len(files), archiveEntryLimit)]
	entries := make([]string, len(files))
	for index, file := range files {
		entries[index] = file.Name
	}
	if uniqueArchiveEntries(entries) == nil {
		return nil, errors.New("archive entry not found")
	}
	for _, file := range files {
		if CleanArchivePath(file.Name) != name || file.UncompressedSize64 > uint64(limit) { //nolint:gosec // Positive limits are validated above.
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return boundedOutput(reader, limit)
	}
	return nil, errors.New("archive entry not found")
}
