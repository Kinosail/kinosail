package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	maxFileSize      = 2 << 20
	maxAuditFileSize = 16 << 20
	maxDecodedSize   = 64 << 20
)

const manifestName = "manifest.json"

type manifest struct {
	Format          int      `json:"format"`
	KinosailVersion string   `json:"kinosailVersion"`
	StateSchema     int      `json:"stateSchema"`
	CreatedAt       string   `json:"createdAt"`
	Files           []string `json:"files"`
	MediaIncluded   bool     `json:"mediaIncluded"`
}

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
	for name := range documents {
		if !service.state.document(name) {
			return fmt.Errorf("read %q: database document is not configured for backup", name)
		}
	}
	names := service.state.written(includeSecrets)
	contents := make(map[string][]byte, len(names))
	written := make([]string, 0, len(names))
	for _, name := range names {
		data, found := documents[name]
		var err error
		if !found {
			data, err = readStateFile(filepath.Join(dataDir, name), service.state.maxSize(name))
		}
		if os.IsNotExist(err) {
			continue
		}
		if found && len(data) > service.state.maxSize(name) {
			return fmt.Errorf("read %s: document exceeds the backup size limit", name)
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
	maximum := maxArchiveSize + entryAllowance(contents, written, service.state.entry)
	if includeSecrets {
		maximum -= encryptedOverhead
	}
	return writeArchiveLimit(writer, version, contents, written, maximum, service.state.entry)
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

func writeArchiveLimit(writer io.Writer, version string, contents map[string][]byte, written []string, maximum int, entry func(string) bool) error {
	manifestData := archiveManifest(version, written)
	if err := validateArchiveSize(contents, written, manifestData, entry); err != nil {
		return err
	}
	output := &archiveBuffer{maximum: maximum}
	gzipWriter := gzip.NewWriter(output)
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
	manifestErr := writeManifestData(tarWriter, manifestData)
	if err := errors.Join(manifestErr, tarWriter.Close(), gzipWriter.Close()); err != nil {
		return err
	}
	return publishArchive(writer, output)
}

func archiveManifest(version string, written []string) []byte {
	data, _ := json.Marshal(manifest{1, version, 1, time.Now().UTC().Format(time.RFC3339), written, false})
	return data
}

func writeManifestData(writer *tar.Writer, data []byte) error {
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

func (service *Service) Restore(reader io.Reader, dataDir string) error {
	staged, err := service.read(reader, false)
	if err != nil {
		return err
	}
	if _, found := staged[auditJournal]; found {
		delete(staged, auditJournal)
		slog.Warn("activity journal not restored from unencrypted backup", "operation", "backup.restore", "outcome", "activity_journal_not_restored", "reason", "journal_key_not_in_archive")
	}
	return service.save(staged, dataDir, service.state.archived(false))
}

func (service *Service) read(reader io.Reader, includeSecrets bool) (map[string][]byte, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	decoded := &io.LimitedReader{R: gzipReader, N: maxDecodedTotal + 1}
	archive := tar.NewReader(decoded)
	staged := make(map[string][]byte)
	whole := int64(1024)
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
		if !service.state.entry(header.Name) {
			whole += archiveEntrySize(int(header.Size))
		}
	}
	if _, err := io.Copy(io.Discard, decoded); err != nil {
		return nil, err
	}
	if decoded.N == 0 || whole > maxDecodedSize {
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
