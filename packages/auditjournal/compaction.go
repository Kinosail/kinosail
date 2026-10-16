package auditjournal

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

func (journal *Journal) persist(event Event, created, now time.Time) (Event, error) {
	if count := len(journal.events); count != 0 {
		event.Previous = journal.events[count-1].Integrity
	}
	event, line := journal.signedLine(event)
	if len(line) > maxAuditEventBytes {
		return event, errors.New("activity event is invalid")
	}
	if now.After(journal.expiry(event.Category, created)) {
		return event, nil
	}
	entry := eventMeta{created: created, size: int64(len(line) + 1)}
	if journal.compactionDue(entry.size, now) {
		return journal.compact(event, entry, now)
	}
	if journal.file != "" {
		if err := appendAuditLine(journal.file, line); err != nil {
			return event, err
		}
	}
	journal.mu.Lock()
	defer journal.mu.Unlock()
	journal.events = append(journal.events, event)
	journal.meta = append(journal.meta, entry)
	journal.bytes += entry.size
	if expiry := journal.expiry(event.Category, created); journal.nextExpiry.IsZero() || expiry.Before(journal.nextExpiry) {
		journal.nextExpiry = expiry
	}
	return event, nil
}

func (journal *Journal) compactionDue(size int64, now time.Time) bool {
	if len(journal.events) >= maxActivityEvents || journal.bytes+size > maxAuditJournalBytes {
		return true
	}
	return !journal.nextExpiry.IsZero() && now.After(journal.nextExpiry) && now.Sub(journal.compacted) >= retentionCompactionInterval
}

func (journal *Journal) compact(event Event, entry eventMeta, now time.Time) (Event, error) {
	count := len(journal.events)
	events, metas := journal.retained(append(journal.events[:count:count], event), append(journal.meta[:count:count], entry), now)
	data, err := journal.encodeChain(events, metas)
	if err != nil {
		return event, err
	}
	if journal.file != "" {
		if err := privatefile.Write(journal.file, data); err != nil {
			return event, err
		}
	}
	journal.mu.Lock()
	defer journal.mu.Unlock()
	journal.events, journal.meta, journal.bytes = events, metas, int64(len(data))
	journal.compacted, journal.nextExpiry = now, journal.earliestExpiry()
	return events[len(events)-1], nil
}

func (journal *Journal) pruneLocked(now time.Time) bool {
	events, metas := journal.retained(journal.events, journal.meta, now)
	pruned := len(events) != len(journal.events)
	journal.events, journal.meta = events, metas
	return pruned
}

func (journal *Journal) retained(events []Event, metas []eventMeta, now time.Time) ([]Event, []eventMeta) {
	keptEvents := make([]Event, 0, len(events))
	keptMetas := make([]eventMeta, 0, len(events))
	for index, event := range events {
		if !now.After(journal.expiry(event.Category, metas[index].created)) {
			keptEvents = append(keptEvents, event)
			keptMetas = append(keptMetas, metas[index])
		}
	}
	first := 0
	if len(keptEvents) > maxActivityEvents {
		first = len(keptEvents) - compactedActivityEvents
	}
	var size int64
	for _, entry := range keptMetas[first:] {
		size += entry.size
	}
	if size > maxAuditJournalBytes {
		for first < len(keptMetas)-1 && size > compactedJournalBytes {
			size -= keptMetas[first].size
			first++
		}
	}
	return keptEvents[first:], keptMetas[first:]
}

func (journal *Journal) rewriteLocked() error {
	if journal.file == "" {
		return nil
	}
	data, err := journal.encodeChain(journal.events, journal.meta)
	if err != nil {
		return err
	}
	if err := privatefile.Write(journal.file, data); err != nil {
		return err
	}
	journal.bytes = int64(len(data))
	return nil
}

func (journal *Journal) encodeChain(events []Event, metas []eventMeta) ([]byte, error) {
	var data bytes.Buffer
	var size int64
	for _, entry := range metas {
		size += entry.size
	}
	data.Grow(int(min(size, maxAuditJournalBytes)))
	previous := ""
	for index := range events {
		events[index].Previous = previous
		event, line := journal.signedLine(events[index])
		events[index] = event
		previous = event.Integrity
		if !journalLineFits(data.Len(), len(line)) {
			return nil, errors.New("activity journal is too large")
		}
		data.Write(line)
		data.WriteByte('\n')
		metas[index].size = int64(len(line) + 1)
	}
	return data.Bytes(), nil
}

func journalLineFits(size, line int) bool {
	return size+line+1 <= maxAuditJournalBytes
}

func (journal *Journal) signedLine(event Event) (Event, []byte) {
	event.Integrity = ""
	data, _ := json.Marshal(event)
	if len(journal.key) == 0 {
		return event, data
	}
	mac := hmac.New(sha256.New, journal.key)
	_, _ = mac.Write(data)
	event.Integrity = hex.EncodeToString(mac.Sum(nil))
	line := make([]byte, 0, len(data)+len(event.Integrity)+len(`,"integrity":""`)+1)
	line = append(line, data[:len(data)-1]...)
	line = append(line, `,"integrity":"`...)
	line = append(line, event.Integrity...)
	return event, append(line, '"', '}')
}

func (journal *Journal) sign(event Event) string {
	event, _ = journal.signedLine(event)
	return event.Integrity
}

func (journal *Journal) expiry(category string, created time.Time) time.Time {
	if category == "playback" {
		return created.Add(journal.playbackRetention)
	}
	return created.Add(journal.auditRetention)
}

func (journal *Journal) earliestExpiry() time.Time {
	var earliest time.Time
	for index, event := range journal.events {
		if expiry := journal.expiry(event.Category, journal.meta[index].created); earliest.IsZero() || expiry.Before(earliest) {
			earliest = expiry
		}
	}
	return earliest
}
