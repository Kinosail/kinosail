package server

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	subtitleUpgradeInterval = 24 * time.Hour
	subtitleUpgradeGain     = 10
)

func (provider *subtitleProvider) upgradeAvailable(item library.Item, language string) bool {
	return provider.configured() && item.Kind == "video" && validLanguage(language)
}

func (provider *subtitleProvider) upgradeEligible(item library.Item, language string, now time.Time) bool { //nolint:cyclop // Eligibility rejects every unsafe managed-file and schedule state.
	if !provider.upgradeAvailable(item, language) {
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
	if !provider.upgradeAvailable(item, language) {
		return false, errors.New("subtitle upgrade is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	target, err := provider.openSidecar(item, language)
	if err != nil {
		return false, err
	}
	defer target.close()
	current, err := target.read("")
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
		return subtitleUpgradeImproves(candidate, previous, managed)
	}
	cleaned, next, err := provider.acquire(ctx, item, language, accept)
	if err != nil {
		observed := subtitleRecord{Source: "external"}
		if managed {
			observed = previous
		}
		observed.CheckedAt = time.Now().Unix()
		observed = target.record(current, observed)
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
		return false, provider.ledger.store(key, target.record(current, next))
	}
	return provider.installSubtitleUpgrade(target, key, current, cleaned.Data, previous, next)
}

func subtitleUpgradeImproves(candidate subtitleDownloadCandidate, previous subtitleRecord, managed bool) bool {
	if managed {
		return candidate.Score >= previous.Score+subtitleUpgradeGain
	}
	return candidate.ExactHash && candidate.Score == 100 && candidate.ReleaseMatch >= 1
}

func (provider *subtitleProvider) installSubtitleUpgrade(target *subtitleSidecar, key string, current, data []byte, previous, next subtitleRecord) (bool, error) {
	if err := provider.retainSubtitleRecoveryOriginal(current, previous, &next); err != nil {
		return false, err
	}
	if backupErr := target.write(".kinosail.bak", current, false); backupErr != nil {
		return false, backupErr
	}
	if err := target.write("", data, false); err != nil {
		return false, err
	}
	next.Backup = true
	if err := provider.ledger.store(key, target.record(data, next)); err != nil {
		_ = target.write("", current, false)
		return false, err
	}
	return true, nil
}

func readUpgradeSidecar(path string) ([]byte, error) {
	return readUpgradeSidecarWith(path, os.Lstat, os.Open)
}

func readUpgradeSidecarWith(path string, lstat func(string) (os.FileInfo, error), open func(string) (*os.File, error)) ([]byte, error) {
	info, err := lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 4<<20 {
		return nil, errors.New("subtitle sidecar cannot be upgraded")
	}
	file, err := open(path)
	if err != nil {
		return nil, errors.New("subtitle sidecar cannot be upgraded")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Size() <= 0 || opened.Size() > 4<<20 {
		return nil, errors.New("subtitle sidecar cannot be upgraded")
	}
	data, err := io.ReadAll(io.LimitReader(file, 4<<20+1))
	if err != nil || len(data) == 0 || len(data) > 4<<20 {
		return nil, errors.New("subtitle sidecar cannot be upgraded")
	}
	return data, nil
}
