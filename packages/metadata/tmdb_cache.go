package metadata

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

var tmdbCacheID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

const maximumLegacyTMDBEntries = 100_000

func MigrateLegacyTMDB(cache string, known func(string) bool, save func(map[string]Record) error, durable bool) bool {
	directory := filepath.Join(cache, "tmdb")
	if cache == "" {
		return false
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		slog.Warn("legacy TMDB cache migration failed", "operation", "metadata.migrate", "failure_class", "read")
		return false
	}
	updates := make(map[string]Record)
	for _, entry := range entries[:min(len(entries), maximumLegacyTMDBEntries)] {
		if !entry.IsDir() || !validTMDBCacheID(entry.Name()) || known(entry.Name()) {
			continue
		}
		if record, ok := legacyTMDBRecord(cache, entry.Name()); ok {
			updates[entry.Name()] = record
		}
	}
	if len(updates) > 0 && save(updates) != nil {
		slog.Warn("legacy TMDB cache migration failed", "operation", "metadata.migrate", "failure_class", "save", "records", len(updates))
		return true
	}
	if durable {
		if err := os.RemoveAll(directory); err != nil {
			slog.Warn("legacy TMDB cache removal failed", "operation", "metadata.migrate", "failure_class", "remove")
		}
	}
	slog.Info("legacy TMDB cache migrated", "operation", "metadata.migrate", "records", len(updates))
	return true
}

func legacyTMDBRecord(cache, id string) (Record, bool) {
	directory := filepath.Join(cache, "tmdb", id)
	data, err := readLegacyFile(filepath.Join(directory, "metadata.json"), 1<<20)
	var legacy TMDBMetadata
	if err != nil || decodeExternalJSON(bytes.NewReader(data), 1<<20, &legacy) != nil || !validCachedTMDB(legacy, directory) {
		return Record{}, false
	}
	record := Record{Title: legacy.Title, Year: legacy.Year, Plot: legacy.Plot, Rating: legacy.Rating, Genres: legacy.Genres, ProviderIDs: map[string]string{"tmdb": strconv.Itoa(legacy.TMDBID)}}
	if legacy.Poster != "" {
		if poster, err := readLegacyFile(legacy.Poster, 8<<20); err == nil {
			target := filepath.Join(cache, "metadata", id+".jpg")
			if privatefile.WriteCache(target, poster) == nil {
				record.Artwork = target
			}
		}
	}
	return record, Bounded(record)
}

func readLegacyFile(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maximum {
		return nil, errors.New("legacy TMDB cache file is invalid")
	}
	file, err := os.Open(path) // #nosec G304 -- fixed legacy cache path beneath the installation cache directory.
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maximum))
}

func validCachedTMDB(metadata TMDBMetadata, directory string) bool {
	if !validTMDBText(metadata) || metadata.Version != 0 && metadata.Version != 2 || metadata.TMDBID <= 0 || len(metadata.Cast) > 15 || !validTMDBCachePath(directory, metadata.Poster) {
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

func validTMDBCacheID(id string) bool {
	return tmdbCacheID.MatchString(id)
}
