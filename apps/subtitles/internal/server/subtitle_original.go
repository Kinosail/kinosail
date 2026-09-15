package server

import (
	"errors"
	"os"
	"path/filepath"
)

func (provider *subtitleProvider) originalPath(fingerprint string) string {
	directory := provider.cache
	if provider.ledger.path != "" {
		directory = filepath.Dir(provider.ledger.path)
	}
	return filepath.Join(directory, "subtitle-originals", fingerprint+".original")
}

func (provider *subtitleProvider) retainSubtitleOriginal(data []byte, record *subtitleRecord) error {
	if len(data) == 0 || len(data) > 4<<20 {
		return errors.New("subtitle original is invalid")
	}
	fingerprint := subtitleFingerprint(data)
	path := provider.originalPath(fingerprint)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errors.New("subtitle original directory is unavailable")
	}
	if err := saveSubtitleExclusive(path, data); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return errors.New("subtitle original could not be retained")
		}
		existing, readErr := readUpgradeSidecar(path)
		if readErr != nil || subtitleFingerprint(existing) != fingerprint {
			return errors.New("subtitle original could not be verified")
		}
	}
	record.OriginalFingerprint = fingerprint
	return nil
}

// Archive a recovery file's original before replacing it. Existing provenance is
// used only when it still describes these exact sidecar bytes.
func (provider *subtitleProvider) retainSubtitleRecoveryOriginal(data []byte, previous subtitleRecord, record *subtitleRecord) error {
	original := data
	if previous.Fingerprint == subtitleFingerprint(data) && previous.OriginalFingerprint != "" {
		var err error
		original, err = readUpgradeSidecar(provider.originalPath(previous.OriginalFingerprint))
		if err != nil || subtitleFingerprint(original) != previous.OriginalFingerprint {
			return errors.New("subtitle recovery original is unavailable")
		}
	}
	var retained subtitleRecord
	if err := provider.retainSubtitleOriginal(original, &retained); err != nil {
		return err
	}
	record.BackupOriginalFingerprint = retained.OriginalFingerprint
	record.BackupFingerprint = subtitleFingerprint(data)
	if previous.Fingerprint == record.BackupFingerprint {
		record.BackupRole = previous.Role
	}
	return nil
}
