package auditjournal

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

func boundedEvent(event Event) Event {
	for _, limit := range [...]int{4096, 256} {
		bounded := boundedEventText(event, limit)
		if data, err := json.Marshal(bounded); err == nil && len(data) <= maxAuditEventBytes/2 {
			return bounded
		}
	}
	return boundedEventText(event, 64)
}

func boundedEventText(event Event, limit int) Event {
	event = cloneEvent(event)
	for _, field := range []*string{&event.Actor, &event.ActorID, &event.Target, &event.TargetID, &event.RequestID, &event.Remote} {
		*field = boundedText(*field, limit)
	}
	for key, value := range event.Details {
		event.Details[key] = boundedText(value, limit)
	}
	return event
}

func boundedText(value string, limit int) string {
	value = strings.ToValidUTF8(strings.ReplaceAll(value, "\x00", "�"), "�")
	if len(value) <= limit {
		return value
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}
