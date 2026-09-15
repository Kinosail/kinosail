package playback

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

const HLSDiagnosticLimit = 8 << 10

type HLSDiagnosticError struct {
	cause  error
	detail string
}

func (failure *HLSDiagnosticError) Error() string  { return "compatible playback failed" }
func (failure *HLSDiagnosticError) Unwrap() error  { return failure.cause }
func (failure *HLSDiagnosticError) Detail() string { return failure.detail }

func NewHLSDiagnosticError(cause error, detail string, private ...string) *HLSDiagnosticError {
	if cause == nil {
		cause = errors.New("compatible playback failed")
	}
	for _, value := range private {
		if value != "" {
			detail = strings.ReplaceAll(detail, value, "<private>")
			if base := filepath.Base(value); base != value {
				detail = strings.ReplaceAll(detail, base, "<private>")
			}
		}
	}
	detail = strings.Join(strings.Fields(detail), " ")
	if detail == "" {
		detail = cause.Error()
	}
	if len(detail) > HLSDiagnosticLimit {
		detail = detail[len(detail)-HLSDiagnosticLimit:]
	}
	return &HLSDiagnosticError{cause: cause, detail: detail}
}

func HLSDiagnostic(err error, private ...string) string {
	if err == nil {
		return ""
	}
	var failure *HLSDiagnosticError
	if errors.As(err, &failure) {
		return failure.detail
	}
	return NewHLSDiagnosticError(err, err.Error(), private...).detail
}

type HLSDiagnosticBuffer struct{ data []byte }

func (buffer *HLSDiagnosticBuffer) Write(data []byte) (int, error) {
	written := len(data)
	buffer.data = append(buffer.data, data...)
	if len(buffer.data) > HLSDiagnosticLimit {
		buffer.data = append(buffer.data[:0], buffer.data[len(buffer.data)-HLSDiagnosticLimit:]...)
	}
	return written, nil
}

func (buffer *HLSDiagnosticBuffer) Bytes() []byte { return append([]byte(nil), buffer.data...) }

func RunHLSCommand(command *exec.Cmd, private ...string) error {
	if command == nil {
		return NewHLSDiagnosticError(errors.New("transcoder command is missing"), "")
	}
	var diagnostic HLSDiagnosticBuffer
	command.Stderr = &diagnostic
	if err := command.Run(); err != nil {
		return NewHLSDiagnosticError(err, string(diagnostic.data), private...)
	}
	return nil
}
