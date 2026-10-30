package server

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"
)

var errSidecarHistory = errors.New("subtitle history could not be saved")

type sidecarInstall struct {
	Key            string
	Data           []byte
	Current        []byte
	Backup         bool
	PreviousBackup []byte
	Record         subtitleRecord
	Action         string
	Reason         string
}

func (provider *subtitleProvider) installSidecar(target *subtitleSidecar, install sidecarInstall) error {
	replacing := install.Current != nil
	if install.Backup && replacing {
		if err := target.write(".kinosail.bak", install.Current, false); err != nil {
			return err
		}
	}
	if err := target.write("", install.Data, !replacing); err != nil {
		provider.rollbackSidecar(target, install, false)
		return err
	}
	if err := provider.ledger.storeHistory(install.Key, target.record(install.Data, install.Record), install.Action, install.Reason); err != nil {
		provider.rollbackSidecar(target, install, true)
		return fmt.Errorf("%w: %w", errSidecarHistory, err)
	}
	slog.Info("subtitle sidecar written", "operation", "subtitle.install", "outcome", install.Action, "reason", install.Reason, "source", install.Record.Source, "key", install.Key)
	return nil
}

func (provider *subtitleProvider) rollbackSidecar(target *subtitleSidecar, install sidecarInstall, written bool) {
	var err error
	switch {
	case written && install.Current != nil:
		err = target.write("", install.Current, false)
	case written:
		err = target.remove()
	}
	if install.Backup && install.Current != nil && install.PreviousBackup != nil {
		err = errors.Join(err, target.write(".kinosail.bak", install.PreviousBackup, false))
	}
	if err != nil {
		slog.Warn("subtitle sidecar rollback incomplete", "operation", "subtitle.install", "outcome", "rollback_failed", "action", install.Action, "key", install.Key, "failure_class", "filesystem")
	}
}

func (file *subtitleSidecar) writeExclusiveDirect(name string, data []byte, mode os.FileMode) error {
	output, err := file.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written := false
	defer func() {
		if !written {
			_ = file.root.Remove(name)
		}
	}()
	if err = output.Chmod(mode); err == nil {
		_, err = output.Write(data)
	}
	if err == nil {
		err = output.Sync()
	}
	if closeErr := output.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	written = true
	file.syncDirectory()
	return nil
}

func hardLinkUnsupported(err error) bool {
	return errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EMLINK)
}
