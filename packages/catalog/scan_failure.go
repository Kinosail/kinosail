package catalog

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"syscall"

	"github.com/MikeO7/kinosail/packages/library"
)

func availabilityError(err error) error {
	if err == nil || errors.Is(err, library.ErrLoading) {
		return err
	}
	return library.ErrUnavailable
}

func logScanFailure(err error, kept bool) {
	class := scanFailureClass(err)
	switch {
	case class == "cancelled":
		slog.Debug("Library scan stopped", "operation", "library.scan", "outcome", "cancelled")
	case kept:
		slog.Warn("Library scan failed; keeping the last Library", "operation", "library.scan", "outcome", "kept", "failure_class", class)
	default:
		slog.Error("Library scan failed; the Library is unavailable", "operation", "library.scan", "outcome", "unavailable", "failure_class", class)
	}
}

func scanFailureClass(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "cancelled"
	case errors.Is(err, fs.ErrNotExist):
		return "missing"
	case errors.Is(err, fs.ErrPermission):
		return "permission"
	case errors.Is(err, syscall.EIO):
		return "io"
	default:
		return "unreadable"
	}
}
