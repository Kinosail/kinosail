package servertransport

import (
	"log"
	"log/slog"
	"regexp"
	"strings"
)

var clientAddress = regexp.MustCompile(`\b(from|serving) (?:\[[^\]]+\]|[^ :]+):\d+`)

type privateHTTPErrorWriter struct{}

func (privateHTTPErrorWriter) Write(message []byte) (int, error) {
	slog.Info("HTTP transport error", "detail", strings.TrimSpace(clientAddress.ReplaceAllString(string(message), "$1 [redacted]")))
	return len(message), nil
}

func privateHTTPErrorLog() *log.Logger { return log.New(privateHTTPErrorWriter{}, "", 0) }
