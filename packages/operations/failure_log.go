package operations

import (
	"strings"
	"sync"
	"time"
)

// FailureEvent is a bounded, private-safe projection of one failed HTTP request.
type FailureEvent struct {
	Time       string `json:"time"`
	Level      string `json:"level"`
	RequestID  string `json:"requestId"`
	Method     string `json:"method"`
	Operation  string `json:"operation"`
	Status     int    `json:"status"`
	DurationMS int64  `json:"durationMs"`
}

// FailureLog keeps recent request failures in memory for Owner diagnostics.
type FailureLog struct {
	mu     sync.Mutex
	events []FailureEvent
}

// Record accepts only safe, bounded fields; raw paths and error text are never retained.
func (log *FailureLog) Record(requestID, method, path string, status int, duration time.Duration) { //nolint:cyclop // The checks bound each field before it enters Owner diagnostics.
	if log == nil || status < 400 || status > 599 {
		return
	}
	level := "warn"
	if status >= 500 {
		level = "error"
	}
	if !safeFailureRequestID(requestID) {
		requestID = ""
	}
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
	default:
		method = "OTHER"
	}
	milliseconds := duration.Milliseconds()
	if milliseconds < 0 {
		milliseconds = 0
	} else if milliseconds > 600_000 {
		milliseconds = 600_000
	}
	event := FailureEvent{time.Now().UTC().Format(time.RFC3339), level, requestID, method, failureOperation(path), status, milliseconds}
	log.mu.Lock()
	defer log.mu.Unlock()
	if len(log.events) == 50 {
		copy(log.events, log.events[1:])
		log.events[49] = event
	} else {
		log.events = append(log.events, event)
	}
}

func safeFailureRequestID(value string) bool {
	if len(value) != 24 && len(value) != 36 {
		return false
	}
	for index := range value {
		if len(value) == 36 && uuidSeparator(index) {
			if value[index] != '-' {
				return false
			}
			continue
		}
		if !hexByte(value[index]) {
			return false
		}
	}
	return true
}

func uuidSeparator(index int) bool { return index == 8 || index == 13 || index == 18 || index == 23 }

func hexByte(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}

// Recent returns a copy in newest-first order.
func (log *FailureLog) Recent() []FailureEvent {
	if log == nil {
		return nil
	}
	log.mu.Lock()
	defer log.mu.Unlock()
	result := make([]FailureEvent, len(log.events))
	for index, event := range log.events {
		result[len(result)-1-index] = event
	}
	return result
}

func failureOperation(rawPath string) string { //nolint:cyclop // Exact operation allowlists keep private path segments out of diagnostics.
	if len(rawPath) > 2048 {
		return "other"
	}
	path, _, _ := strings.Cut(rawPath, "?")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "v1" {
		if parts[2] == "items" && len(parts) >= 5 {
			switch parts[4] {
			case "playback", "playback-preferences", "progress", "list", "reader":
				return "items-" + parts[4]
			}
		}
		switch parts[2] {
		case "items", "library", "me", "session", "quick-connect", "shows", "albums", "collections", "books", "cast", "remote-players", "downloads", "diagnostics":
			return parts[2]
		default:
			return "api-other"
		}
	}
	if len(parts) > 0 {
		switch parts[0] {
		case "hls", "media", "art":
			return parts[0]
		}
	}
	return "other"
}
