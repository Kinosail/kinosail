package server

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	subtitleUpgradeInterval = 24 * time.Hour
	subtitleUpgradeGain     = 10
)

func (provider *subtitleProvider) upgradeEligible(item library.Item, language string, now time.Time) bool { //nolint:cyclop // Eligibility rejects every unsafe managed-file and schedule state.
	if !provider.configured() || item.Kind != "video" || !validLanguage(language) {
		return false
	}
	target := subtitleSidecarPath(item, language)
	info, err := os.Lstat(target)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 4<<20 {
		return false
	}
	record, found, err := provider.ledger.record(subtitleRecordKey(item.ID, language))
	if err != nil {
		return false
	}
	unchanged := found && record.Size == info.Size() && record.Modified == info.ModTime().UnixNano()
	if unchanged && record.Frozen {
		return false
	}
	if unchanged && record.Managed && record.Score == 100 || unchanged && now.Sub(time.Unix(record.CheckedAt, 0)) < subtitleUpgradeInterval {
		return false
	}
	return true
}

func (provider *subtitleProvider) upgradeSidecar(ctx context.Context, item library.Item, language string) (bool, error) { //nolint:cyclop,gocognit // Upgrade validation, backup, write, and rollback form one atomic operation.
	provider.sidecar.Lock()
	defer provider.sidecar.Unlock()
	if !provider.configured() || item.Kind != "video" || !validLanguage(language) {
		return false, errors.New("subtitle upgrade is unavailable")
	}
	target := subtitleSidecarPath(item, language)
	current, err := readUpgradeSidecar(target)
	if err != nil {
		return false, err
	}
	key, fingerprint := subtitleRecordKey(item.ID, language), subtitleFingerprint(current)
	previous, found, ledgerErr := provider.ledger.record(key)
	managed := ledgerErr == nil && found && previous.Fingerprint == fingerprint && previous.Managed
	if ledgerErr != nil || found && previous.Fingerprint == fingerprint && previous.Frozen || managed && previous.Score == 100 {
		return false, ledgerErr
	}
	accept := func(candidate subtitleDownloadCandidate) bool {
		if managed {
			return candidate.Score >= previous.Score+subtitleUpgradeGain
		}
		return candidate.ExactHash && candidate.Score == 100 && candidate.ReleaseMatch >= 1
	}
	cleaned, next, err := provider.acquire(ctx, item, language, accept)
	if err != nil {
		observed := subtitleRecord{Source: "external"}
		if managed {
			observed = previous
		}
		observed.CheckedAt = time.Now().Unix()
		observed = completeSubtitleRecord(target, current, observed)
		_ = provider.ledger.store(key, observed)
		return false, err
	}
	if err = provider.retainSubtitleOriginal(cleaned.Original, &next); err != nil {
		return false, err
	}
	next.Fingerprint = subtitleFingerprint(cleaned.Data)
	if next.Fingerprint == fingerprint {
		if found && previous.Fingerprint == fingerprint {
			next.Backup, next.BackupOriginalFingerprint, next.BackupFingerprint, next.BackupRole = previous.Backup, previous.BackupOriginalFingerprint, previous.BackupFingerprint, previous.BackupRole
		}
		return false, provider.ledger.store(key, completeSubtitleRecord(target, current, next))
	}
	if err = provider.retainSubtitleRecoveryOriginal(current, previous, &next); err != nil {
		return false, err
	}
	if backupErr := saveSubtitle(target+".kinosail.bak", current); backupErr != nil {
		return false, backupErr
	}
	if err = saveSubtitle(target, cleaned.Data); err != nil {
		return false, err
	}
	next.Backup = true
	if err = provider.ledger.store(key, completeSubtitleRecord(target, cleaned.Data, next)); err != nil {
		_ = saveSubtitle(target, current)
		return false, err
	}
	return true, nil
}

func readUpgradeSidecar(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 4<<20 {
		return nil, errors.New("subtitle sidecar cannot be upgraded")
	}
	data, err := os.ReadFile(path) //nolint:gosec // The path is a scanned media sidecar.
	if err != nil || len(data) == 0 || len(data) > 4<<20 {
		return nil, errors.New("subtitle sidecar cannot be upgraded")
	}
	return data, nil
}
