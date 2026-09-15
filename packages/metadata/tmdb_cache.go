package metadata

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

var tmdbCacheID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func (client *TMDBClient) Active() bool {
	if client == nil || client.cacheDir == "" {
		return client != nil && client.token != ""
	}
	return client.token != "" || HasTMDBCache(client.cacheDir)
}

func HasTMDBCache(directory string) bool {
	root, err := os.Open(directory) //nolint:gosec // The installation owns the configured cache directory.
	if err != nil {
		return false
	}
	defer root.Close()
	for {
		entries, err := root.ReadDir(100)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			//nolint:gosec // ReadDir returns one base name below the installation-owned cache directory.
			info, statErr := os.Lstat(filepath.Join(directory, entry.Name(), "metadata.json"))
			if statErr == nil && info.Mode().IsRegular() {
				return true
			}
		}
		if err != nil {
			return false
		}
	}
}

func (client *TMDBClient) Load(id string) (TMDBMetadata, bool) {
	if client == nil || !validTMDBCacheID(id) {
		return TMDBMetadata{}, false
	}
	path := client.metadataPath(id)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 1<<20 {
		return TMDBMetadata{}, false
	}
	file, err := os.Open(path) // #nosec G304 -- fixed cache path beneath the installation cache directory.
	if err != nil {
		return TMDBMetadata{}, false
	}
	defer file.Close()
	var metadata TMDBMetadata
	if decodeExternalJSON(file, 1<<20, &metadata) != nil || !validCachedTMDB(metadata, filepath.Dir(path)) {
		return TMDBMetadata{}, false
	}
	return metadata, time.Since(info.ModTime()) < 7*24*time.Hour
}

func validCachedTMDB(metadata TMDBMetadata, directory string) bool {
	if !validTMDBText(metadata) || metadata.TMDBID <= 0 || len(metadata.Cast) > 15 || !validTMDBCachePath(directory, metadata.Poster) {
		return false
	}
	return validCachedTMDBCast(metadata.Cast, directory)
}

func validCachedTMDBCast(cast []library.Person, directory string) bool {
	for _, person := range cast {
		if person.Name == "" || len(person.Name) > 200 || len(person.Role) > 200 || hasControlText(person.Name+person.Role) || !validTMDBCachePath(directory, person.Image) {
			return false
		}
	}
	return true
}

func validTMDBCachePath(directory, path string) bool {
	if path == "" {
		return true
	}
	relative, err := filepath.Rel(directory, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func (client *TMDBClient) metadataPath(id string) string {
	return filepath.Join(client.cacheDir, id, "metadata.json")
}

func validTMDBCacheID(id string) bool {
	return tmdbCacheID.MatchString(id)
}
