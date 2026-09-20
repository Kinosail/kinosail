package server

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
)

// subtitleSidecar retains the authorized parent throughout acquisition and rollback.
type subtitleSidecar struct {
	root *os.Root
	name string
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
			return &subtitleSidecar{parent, filepath.Base(subtitleSidecarPath(item, language))}, nil
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
	temporary := ".kinosail-subtitle-" + rand.Text()
	output, err := file.root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = output.Close(); _ = file.root.Remove(temporary) }()
	if _, err = output.Write(data); err != nil {
		return err
	}
	if err = output.Close(); err != nil {
		return err
	}
	if exclusive {
		return file.root.Link(temporary, name)
	}
	return file.root.Rename(temporary, name)
}

func (file *subtitleSidecar) record(data []byte, record subtitleRecord) subtitleRecord {
	record.Fingerprint = subtitleFingerprint(data)
	if info, err := file.root.Stat(file.name); err == nil {
		record.Size, record.Modified = info.Size(), info.ModTime().UnixNano()
	}
	return record
}

func (file *subtitleSidecar) remove() error { return file.root.Remove(file.name) }
