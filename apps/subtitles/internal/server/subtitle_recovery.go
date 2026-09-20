package server

import (
	"errors"
	"os"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func (provider *subtitleProvider) setReplacement(item library.Item, language string, replaceable bool) error {
	provider.sidecar.Lock()
	defer provider.sidecar.Unlock()
	if item.Kind != "video" || !validLanguage(language) {
		return errors.New("subtitle replacement policy is invalid")
	}
	target, err := provider.openSidecar(item, language)
	if err != nil {
		return err
	}
	defer target.close()
	data, err := target.read("")
	if err != nil {
		return err
	}
	key := subtitleRecordKey(item.ID, language)
	record, found, ledgerErr := provider.ledger.record(key)
	if ledgerErr != nil {
		return ledgerErr
	}
	if !found || record.Fingerprint != subtitleFingerprint(data) {
		now := time.Now().Unix()
		record = subtitleRecord{Source: "external", CheckedAt: now, InstalledAt: now, Synchronization: "none"}
	}
	record.Frozen = !replaceable
	record = target.record(data, record)
	return provider.ledger.store(key, record)
}

func (provider *subtitleProvider) restorePrevious(item library.Item, language string) error {
	provider.sidecar.Lock()
	defer provider.sidecar.Unlock()
	if item.Kind != "video" || !validLanguage(language) {
		return errors.New("subtitle restore request is invalid")
	}
	target, err := provider.openSidecar(item, language)
	if err != nil {
		return err
	}
	defer target.close()
	current, err := target.read("")
	if err != nil {
		return err
	}
	previous, err := target.read(".kinosail.bak")
	if err != nil {
		return errors.New("previous subtitle is unavailable")
	}
	key := subtitleRecordKey(item.ID, language)
	currentRecord, found, ledgerErr := provider.ledger.record(key)
	if ledgerErr != nil {
		return ledgerErr
	}
	if !found || currentRecord.Fingerprint != subtitleFingerprint(current) {
		currentRecord = subtitleRecord{}
	}
	now := time.Now().Unix()
	record := subtitleRecord{Source: "external", CheckedAt: now, InstalledAt: now, Managed: false, Cleanup: []string{"Restored previous subtitle"}, Synchronization: "none", Frozen: true, Backup: true}
	if err = provider.retainSubtitleRecoveryOriginal(current, currentRecord, &record); err != nil {
		return err
	}
	if currentRecord.BackupFingerprint == subtitleFingerprint(previous) {
		record.Role = currentRecord.BackupRole
	}
	original := previous
	if currentRecord.BackupOriginalFingerprint != "" && currentRecord.BackupFingerprint == subtitleFingerprint(previous) {
		original, err = readUpgradeSidecar(provider.originalPath(currentRecord.BackupOriginalFingerprint))
		if err != nil || subtitleFingerprint(original) != currentRecord.BackupOriginalFingerprint {
			return errors.New("subtitle recovery original is unavailable")
		}
	}
	if err = provider.retainSubtitleOriginal(original, &record); err != nil {
		return err
	}
	if err = target.write("", previous, false); err != nil {
		return err
	}
	if err = target.write(".kinosail.bak", current, false); err != nil {
		_ = target.write("", current, false)
		return err
	}
	record = target.record(previous, record)
	if err = provider.ledger.store(subtitleRecordKey(item.ID, language), record); err != nil {
		_ = target.write("", current, false)
		_ = target.write(".kinosail.bak", previous, false)
		return err
	}
	return nil
}

func subtitleBackupAvailable(item library.Item, language string) bool {
	info, err := os.Lstat(subtitleSidecarPath(item, language) + ".kinosail.bak")
	return err == nil && info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= 4<<20
}
