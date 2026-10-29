package backup

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"syscall"
	"time"
)

var backupFailureMessages = map[string]string{
	"permission-denied":    "Kinosail cannot write to the backup folder. Check the folder permissions.",
	"read-only-filesystem": "The backup folder is read-only. Mount it with write access.",
	"no-space":             "The backup folder has no free space.",
	"invalid-path":         "The backup folder path is not a folder Kinosail can use.",
}

func backupFailureClass(err error) string {
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, syscall.EROFS):
		return "read-only-filesystem"
	case errors.Is(err, fs.ErrPermission):
		return "permission-denied"
	case errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT):
		return "no-space"
	case errors.Is(err, syscall.ENOTDIR) || errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENAMETOOLONG):
		return "invalid-path"
	}
	return "write-failed"
}

type backupError struct{ cause error }

func (failure backupError) Error() string { return backupFailureMessage(failure.cause) }

func (failure backupError) Unwrap() error { return failure.cause }

func backupFailureMessage(err error) string {
	if message, found := backupFailureMessages[backupFailureClass(err)]; found {
		return message
	}
	return err.Error()
}

func (manager *Manager) reportScheduled(ctx context.Context, err error, previous string, delay time.Duration) (string, time.Duration) {
	if err == nil {
		if previous != "" {
			slog.Info("automatic backup recovered", "operation", "backup.automatic", "previous_failure_class", previous)
		}
		slog.Info("automatic backup completed", "encrypted", manager.key != "", "retention", manager.retention)
		return "", manager.interval
	}
	class := backupFailureClass(err)
	if manager.validate() != nil {
		class = "not-configured"
	}
	if class == "canceled" && ctx.Err() != nil {
		return previous, delay
	}
	delay = manager.retryDelay(delay, class == previous)
	level := slog.LevelWarn
	if class == previous {
		level = slog.LevelDebug
	}
	slog.Log(ctx, level, "automatic backup failed", "operation", "backup.automatic", "failure_class", class, "retry_in", delay.String())
	return class, delay
}
