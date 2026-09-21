// Package backup archives installation state without Library Content or cache data.
package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const (
	maxFileSize      = 2 << 20
	maxAuditFileSize = 16 << 20
	maxDecodedSize   = 64 << 20
)

const manifestName = "manifest.json"

var (
	files       = []string{"api_keys.json", "audit.jsonl", "collections.json", "configuration.json", "history.json", "lists.json", "metadata.json", "playback_markers.json", "playlist_order.json", "playlists.json", "profiles.json", "progress.json", "server-identity.json", "sessions.json", "settings.json", "smart_playlists.json"}
	secretFiles = []string{"audit_key.json", "secrets.json", "viewing_imports.json"}
)

type manifest struct {
	Format          int      `json:"format"`
	KinosailVersion string   `json:"kinosailVersion"`
	StateSchema     int      `json:"stateSchema"`
	CreatedAt       string   `json:"createdAt"`
	Files           []string `json:"files"`
	MediaIncluded   bool     `json:"mediaIncluded"`
}

// Write creates a compressed archive of Kinosail's configuration state.
func (service *Service) Write(writer io.Writer, dataDir, version string) error {
	return service.write(writer, dataDir, version, false)
}

func (service *Service) write(writer io.Writer, dataDir, version string, includeSecrets bool) error {
	if !validVersion(version) {
		return errors.New("invalid application version")
	}
	documents, err := service.readDocuments(dataDir)
	if err != nil {
		return err
	}
	names := files
	if includeSecrets {
		names = append(append([]string(nil), files...), secretFiles...)
	}
	contents := make(map[string][]byte, len(names))
	written := make([]string, 0, len(names))
	for _, name := range names {
		data, found := documents[name]
		var err error
		if !found {
			data, err = readStateFile(filepath.Join(dataDir, name), maxStateFileSize(name))
		}
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !service.validState(name, data) {
			return fmt.Errorf("read %s: invalid configuration file", name)
		}
		contents[name] = data
		written = append(written, name)
	}
	if len(written) == 0 {
		return errors.New("no configuration state to back up")
	}
	return writeArchive(writer, version, contents, written)
}

func (service *Service) readDocuments(dataDir string) (map[string][]byte, error) {
	info, err := os.Lstat(filepath.Join(dataDir, service.databaseFilename))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("invalid SQLite database file")
	}
	store, err := service.openDatabase(dataDir)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("database is unavailable")
	}
	documents, exportErr := store.Export()
	return documents, errors.Join(exportErr, store.Close())
}

func writeArchive(writer io.Writer, version string, contents map[string][]byte, written []string) error {
	gzipWriter := gzip.NewWriter(writer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, name := range written {
		data := contents[name]
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}); err != nil {
			return err
		}
		if _, err := tarWriter.Write(data); err != nil {
			return err
		}
	}
	manifestErr := writeArchiveManifest(tarWriter, version, written)
	return errors.Join(manifestErr, tarWriter.Close(), gzipWriter.Close())
}

func writeArchiveManifest(writer *tar.Writer, version string, written []string) error {
	data, _ := json.Marshal(manifest{1, version, 1, time.Now().UTC().Format(time.RFC3339), written, false})
	if err := writer.WriteHeader(&tar.Header{Name: manifestName, Mode: 0o600, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

func readStateFile(path string, maximum int) ([]byte, error) {
	return readStateFileWith(path, maximum, os.Lstat, os.Open)
}

func readStateFileWith(path string, maximum int, lstat func(string) (os.FileInfo, error), open func(string) (*os.File, error)) ([]byte, error) {
	info, err := lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(maximum) {
		return nil, errors.New("invalid state file")
	}
	file, err := open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Size() > int64(maximum) {
		return nil, errors.New("invalid state file")
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil || len(data) > maximum {
		return nil, errors.New("invalid state file")
	}
	return data, nil
}

// Restore validates and atomically replaces configuration files from an archive.
func (service *Service) Restore(reader io.Reader, dataDir string) error {
	staged, err := service.read(reader, false)
	if err != nil {
		return err
	}
	return service.save(staged, dataDir, files)
}

func (service *Service) read(reader io.Reader, includeSecrets bool) (map[string][]byte, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	decoded := &io.LimitedReader{R: gzipReader, N: maxDecodedSize + 1}
	archive := tar.NewReader(decoded)
	staged := make(map[string][]byte)
	for {
		header, nextErr := archive.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return nil, nextErr
		}
		if err := service.stage(archive, header, staged, includeSecrets); err != nil {
			return nil, err
		}
	}
	if _, err := io.Copy(io.Discard, decoded); err != nil {
		return nil, err
	}
	if decoded.N == 0 {
		return nil, errors.New("backup exceeds maximum decoded size")
	}
	if err := validateManifest(staged); err != nil {
		return nil, err
	}
	if len(staged) == 0 {
		return nil, errors.New("backup contains no configuration state")
	}
	return staged, nil
}

func (service *Service) stage(archive *tar.Reader, header *tar.Header, staged map[string][]byte, includeSecrets bool) error { //nolint:cyclop // Entry safety checks deliberately stay at the archive boundary.
	allowed := slices.Contains(files, header.Name) || includeSecrets && slices.Contains(secretFiles, header.Name) || header.Name == manifestName
	maximum := maxStateFileSize(header.Name)
	if header.Typeflag != tar.TypeReg || !filepath.IsLocal(header.Name) || filepath.Base(header.Name) != header.Name || !allowed || header.Size < 0 || header.Size > int64(maximum) {
		return fmt.Errorf("backup contains invalid entry %q", header.Name)
	}
	if _, exists := staged[header.Name]; exists {
		return fmt.Errorf("backup contains duplicate entry %q", header.Name)
	}
	data, err := io.ReadAll(io.LimitReader(archive, int64(maximum+1)))
	if err != nil || int64(len(data)) != header.Size || !service.validState(header.Name, data) {
		return fmt.Errorf("backup contains invalid %s", header.Name)
	}
	staged[header.Name] = data
	return nil
}

func maxStateFileSize(name string) int {
	if name == "audit.jsonl" {
		return maxAuditFileSize
	}
	if slices.Contains(databaseDocuments, name) {
		return databaseDocumentLimit
	}
	return maxFileSize
}

func (service *Service) validState(name string, data []byte) bool {
	if name != "audit.jsonl" {
		return json.Valid(data) && (name != "settings.json" || service.validateSettings(data) == nil)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	if len(lines) == 1 && len(lines[0]) == 0 {
		return false
	}
	for _, line := range lines {
		if !json.Valid(line) {
			return false
		}
	}
	return true
}

func validateManifest(staged map[string][]byte) error { //nolint:cyclop // Every manifest invariant is part of one fail-closed boundary.
	data, exists := staged[manifestName]
	if !exists {
		return nil
	}
	delete(staged, manifestName)
	var description manifest
	if json.Unmarshal(data, &description) != nil || description.Format != 1 || description.StateSchema != 1 || description.KinosailVersion == "" {
		return errors.New("backup manifest is invalid")
	}
	if _, err := time.Parse(time.RFC3339, description.CreatedAt); err != nil {
		return errors.New("backup manifest is invalid")
	}
	actual := make([]string, 0, len(staged))
	for name := range staged {
		actual = append(actual, name)
	}
	slices.Sort(actual)
	slices.Sort(description.Files)
	if !slices.Equal(actual, description.Files) || description.MediaIncluded {
		return errors.New("backup manifest does not match archive")
	}
	return nil
}
