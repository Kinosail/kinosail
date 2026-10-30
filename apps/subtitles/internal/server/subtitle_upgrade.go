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

func (provider *subtitleProvider) upgradeEligible(item library.Item, language string, now time.Time) bool {
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
	if !unchanged || !record.Managed || record.Frozen || record.Score == 100 {
		return false
	}
	return now.Sub(time.Unix(record.CheckedAt, 0)) >= subtitleUpgradeInterval
}

func (provider *subtitleProvider) upgradeSidecar(ctx context.Context, item library.Item, language string) (bool, error) {
	ctx, settled, admissionErr := provider.admission.enter(ctx)
	if admissionErr != nil {
		return false, admissionErr
	}
	defer settled()
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
	if ledgerErr != nil || !managed || previous.Frozen || previous.Score == 100 {
		return false, ledgerErr
	}
	accept := func(candidate subtitleDownloadCandidate) bool {
		return subtitleUpgradeImproves(candidate, previous)
	}
	cleaned, next, err := provider.acquire(ctx, item, language, accept)
	provider.sidecar.Lock()
	defer provider.sidecar.Unlock()
	if !provider.sidecarUnchanged(target, key, fingerprint, previous, found) {
		return false, nil
	}
	if err != nil {
		provider.recordUpgradeCheck(target, key, current, previous) //nolint:contextcheck // Ledger commits must finish after client cancellation.
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
	return provider.installSubtitleUpgrade(target, key, current, cleaned.Data, previous, next, managed)
}

func (provider *subtitleProvider) recordUpgradeCheck(target *subtitleSidecar, key string, current []byte, previous subtitleRecord) {
	observed := previous
	observed.CheckedAt = time.Now().Unix()
	_ = provider.ledger.store(key, target.record(current, observed))
}

func (provider *subtitleProvider) sidecarUnchanged(target *subtitleSidecar, key, fingerprint string, previous subtitleRecord, found bool) bool {
	latest, err := target.read("")
	if err != nil || subtitleFingerprint(latest) != fingerprint {
		return false
	}
	record, recorded, err := provider.ledger.record(key)
	return err == nil && recorded == found && record.Fingerprint == previous.Fingerprint && record.CheckedAt == previous.CheckedAt && record.Frozen == previous.Frozen && record.InstalledAt == previous.InstalledAt
}

func subtitleUpgradeImproves(candidate subtitleDownloadCandidate, previous subtitleRecord) bool {
	return candidate.Score >= previous.Score+subtitleUpgradeGain
}

func (provider *subtitleProvider) installSubtitleUpgrade(target *subtitleSidecar, key string, current, data []byte, previous, next subtitleRecord, managed bool) (bool, error) {
	if err := provider.retainSubtitleRecoveryOriginal(current, previous, &next); err != nil {
		return false, err
	}
	next.Backup = true
	reason := "exact-hash"
	if managed {
		reason = "higher-score"
	}
	if err := provider.installSidecar(target, sidecarInstall{Key: key, Data: data, Current: current, Backup: true, Record: next, Action: "updated", Reason: reason}); err != nil {
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
