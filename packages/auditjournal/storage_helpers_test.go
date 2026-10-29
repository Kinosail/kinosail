package auditjournal

import "encoding/json"

func appendAuditEvent(path string, event Event) error {
	line, _ := json.Marshal(event)
	return appendAuditLine(path, line)
}
