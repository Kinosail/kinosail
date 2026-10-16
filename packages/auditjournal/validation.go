package auditjournal

import (
	"encoding/json"
	"strings"
	"time"
)

func validEvent(event Event, stored bool) bool {
	if !validCoreFields(event) || !validOptionalFields(event) || !validDetails(event.Details) {
		return false
	}
	if !stored && (event.Previous != "" || event.Integrity != "") {
		return false
	}
	data, err := json.Marshal(event)
	return err == nil && len(data) <= maxAuditEventBytes
}

func validCoreFields(event Event) bool {
	if !validRequiredString(event.ID, 128) || !validRequiredString(event.Time, 64) || !validRequiredString(event.Category, 64) || !validRequiredString(event.Action, 256) || !validRequiredString(event.Result, 64) {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, event.Time)
	return err == nil
}

func validOptionalFields(event Event) bool {
	for _, value := range []string{event.Actor, event.ActorID, event.Target, event.TargetID, event.RequestID, event.Remote} {
		if !validOptionalString(value, 4096) {
			return false
		}
	}
	return true
}

func validRequiredString(value string, limit int) bool {
	return value != "" && validOptionalString(value, limit)
}

func validOptionalString(value string, limit int) bool {
	return len(value) <= limit && !strings.ContainsRune(value, 0)
}

func validDetails(details map[string]string) bool {
	if len(details) > 128 {
		return false
	}
	for key, value := range details {
		if !validRequiredString(key, 256) || !validOptionalString(value, 4096) {
			return false
		}
	}
	return true
}
