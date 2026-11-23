package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"syscall"
)

type startupFailure struct {
	message string
	stage   string
	err     error
}

func unavailableApplication(config Config, failure startupFailure) http.Handler {
	if config.InternetAccess != nil {
		_ = config.InternetAccess.Kill()
	}
	attributes := []any{"operation", "startup", "outcome", "unavailable", "stage", failure.stage, "failure_class", startupFailureClass(failure.err)}
	var path *fs.PathError
	if errors.As(failure.err, &path) {
		attributes = append(attributes, "failure_op", path.Op)
	}
	if failure.err != nil {
		attributes = append(attributes, "error_type", innermostErrorType(failure.err))
	}
	slog.Error("Kinosail could not start", attributes...)
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, failure.message, http.StatusServiceUnavailable)
	})
}

func startupFailureClass(err error) string {
	switch {
	case err == nil:
		return "unhealthy"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "cancelled"
	case errors.Is(err, fs.ErrPermission):
		return "permission"
	case errors.Is(err, fs.ErrNotExist):
		return "missing"
	case errors.Is(err, syscall.ENOTDIR):
		return "not-directory"
	case errors.Is(err, fs.ErrInvalid):
		return "invalid"
	default:
		return "failed"
	}
}

func innermostErrorType(err error) string {
	for next := errors.Unwrap(err); next != nil; next = errors.Unwrap(err) {
		err = next
	}
	return fmt.Sprintf("%T", err)
}
