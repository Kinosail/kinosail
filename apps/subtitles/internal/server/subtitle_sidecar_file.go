package server

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleSidecar struct {
	root  *os.Root
	name  string
	media string
}

func (provider *subtitleProvider) openSidecar(item library.Item, language string) (*subtitleSidecar, error) {
	if provider.index == nil || item.Kind != "video" || !validLanguage(language) || !filepath.IsAbs(item.Path) {
		return nil, errors.New("subtitle destination is unavailable")
	}
	for _, allowed := range provider.index.Roots() {
		relative, err := filepath.Rel(allowed.Path, filepath.Dir(item.Path))
		if err != nil || !filepath.IsLocal(relative) {
			continue
		}
		root, err := os.OpenRoot(allowed.Path)
		if err != nil {
			continue
		}
		parent, err := root.OpenRoot(relative)
		_ = root.Close()
		if err == nil {
			return &subtitleSidecar{parent, filepath.Base(subtitleSidecarPath(item, language)), filepath.Base(item.Path)}, nil
		}
	}
	return nil, errors.New("subtitle destination is outside the Library")
}

func (file *subtitleSidecar) close() { _ = file.root.Close() }

func (file *subtitleSidecar) read(suffix string) ([]byte, error) {
	return readUpgradeSidecarWith(file.name+suffix, file.root.Lstat, file.root.Open)
}

func (file *subtitleSidecar) write(suffix string, data []byte, exclusive bool) error {
	name := file.name + suffix
	file.removeStaleTemporaries(time.Now())
	temporary := sidecarTemporaryPrefix + rand.Text()
	output, err := file.root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err = output.Chmod(file.mode(name)); err != nil {
		_ = output.Close()
		_ = file.root.Remove(temporary)
		return err
	}
	defer func() { _ = output.Close(); _ = file.root.Remove(temporary) }()
	if _, err = output.Write(data); err != nil {
		return err
	}
	if err = output.Sync(); err != nil {
		return err
	}
	if err = output.Close(); err != nil {
		return err
	}
	if exclusive {
		err = file.root.Link(temporary, name)
		if hardLinkUnsupported(err) {
			return file.writeExclusiveDirect(name, data, file.mode(name))
		}
	} else {
		err = file.root.Rename(temporary, name)
	}
	if err == nil {
		file.syncDirectory()
	}
	return err
}

func (file *subtitleSidecar) syncDirectory() {
	if directory, err := file.root.Open("."); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
}

func (file *subtitleSidecar) record(data []byte, record subtitleRecord) subtitleRecord {
	record.Fingerprint = subtitleFingerprint(data)
	if info, err := file.root.Stat(file.name); err == nil {
		record.Size, record.Modified = info.Size(), info.ModTime().UnixNano()
	}
	return record
}

func (file *subtitleSidecar) remove() error { return file.root.Remove(file.name) }

const sidecarTemporaryPrefix = ".kinosail-subtitle-"

func (file *subtitleSidecar) mode(name string) os.FileMode {
	if info, err := file.root.Lstat(name); err == nil && info.Mode().IsRegular() {
		return info.Mode().Perm()
	}
	if info, err := file.root.Stat(file.media); err == nil && info.Mode().IsRegular() {
		return info.Mode().Perm() & 0o666
	}
	return 0o644
}

func (file *subtitleSidecar) removeStaleTemporaries(now time.Time) {
	directory, err := file.root.Open(".")
	if err != nil {
		return
	}
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), sidecarTemporaryPrefix) || !entry.Type().IsRegular() {
			continue
		}
		if info, infoErr := entry.Info(); infoErr == nil && now.Sub(info.ModTime()) > time.Hour {
			_ = file.root.Remove(entry.Name())
		}
	}
}
