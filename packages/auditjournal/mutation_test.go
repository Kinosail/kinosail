package auditjournal

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNotificationLifecyclePredicatesAreIndependent(t *testing.T) {
	delivered := make(chan Event, 1)
	journal := New(nil, Config{Notify: func(event Event) { delivered <- event }}) //nolint:staticcheck // Nil context is an explicitly supported compatibility input.
	if journal.notifications == nil {
		t.Fatal("nil context disabled notifications")
	}
	journal.Record(testEvent("playback", "playback", time.Now().UTC()))
	select {
	case event := <-delivered:
		t.Fatalf("playback event was delivered: %#v", event)
	case <-time.After(10 * time.Millisecond):
	}
	journal.Record(testEvent("security", "security", time.Now().UTC()))
	waitForNotification(t, eventIDs(delivered), "security")
	invalid := New(t.Context(), Config{DataDir: "invalid\x00directory", Notify: func(Event) {}})
	if invalid.notifications != nil {
		t.Fatal("unwritable journal started notification delivery")
	}
	withoutNotify := New(t.Context(), Config{})
	if withoutNotify.notifications != nil {
		t.Fatal("nil notifier started notification delivery")
	}
}

func eventIDs(events <-chan Event) <-chan string {
	result := make(chan string, 1)
	go func() { result <- (<-events).ID }()
	return result
}

func TestQueryLogTripwireAndStatusExactBounds(t *testing.T) {
	now := time.Now().UTC()
	journal := New(t.Context(), Config{})
	journal.Record(testEvent("first", "security", now))
	journal.Record(testEvent("second", "security", now))
	if events := journal.Query("", 1); len(events) != 1 || events[0].ID != "second" {
		t.Fatalf("limited query = %#v", events)
	}
	journal = New(t.Context(), Config{})
	for index := range 26 {
		journal.Record(testEvent(string(rune('a'+index)), "security", now))
	}
	if logs := journal.RecentLogs(); len(logs) != 25 || logs[0].ID != "z" || logs[24].ID != "b" {
		t.Fatalf("recent logs = %#v", logs)
	}
	tripwire := New(t.Context(), Config{})
	event := testEvent("tripwire", "security", now)
	event.Action = "security.tripwire"
	tripwire.Record(event)
	tripwire.Record(testEvent("later", "security", now))
	if tripwire.TripwireWarning() == "" {
		t.Fatal("oldest tripwire was skipped")
	}
	queued := &Journal{notifications: make(chan Event, 1)}
	queued.notifications <- event
	if status := queued.Status(); status.NotificationQueued != 1 || !status.Healthy {
		t.Fatalf("queued status = %#v", status)
	}
}

func TestEventValidationExactBoundsAndMarshalLimit(t *testing.T) { //nolint:cyclop // Exact field and encoded-size boundaries share one event fixture.
	now := time.Now().UTC()
	event := testEvent(strings.Repeat("i", 128), strings.Repeat("c", 64), now)
	event.Action = strings.Repeat("a", 256)
	event.Result = strings.Repeat("r", 64)
	event.Actor = strings.Repeat("x", 4096)
	event.Details = make(map[string]string, 128)
	for index := range 128 {
		event.Details[string(rune(index+1))] = ""
	}
	if !validEvent(event, false) {
		t.Fatal("exact event bounds were rejected")
	}
	exact := testEvent("exact", "security", now)
	exact.Details = make(map[string]string, 16)
	for index := range 15 {
		exact.Details[string(rune('a'+index))] = strings.Repeat("x", 4096)
	}
	exact.Details["final"] = ""
	encoded, _ := json.Marshal(exact)
	remaining := maxAuditEventBytes - len(encoded)
	if remaining < 0 || remaining > 4096 {
		t.Fatalf("exact event remainder = %d", remaining)
	}
	exact.Details["final"] = strings.Repeat("x", remaining)
	encoded, _ = json.Marshal(exact)
	if len(encoded) != maxAuditEventBytes || !validEvent(exact, false) {
		t.Fatalf("exact encoded event length = %d", len(encoded))
	}
	if err := appendAuditEvent(filepath.Join(t.TempDir(), "exact.jsonl"), exact); err != nil {
		t.Fatalf("append exact encoded event: %v", err)
	}
	if validDetails(map[string]string{"": "value"}) || validDetails(map[string]string{"key": strings.Repeat("x", 4097)}) {
		t.Fatal("independent detail bounds were accepted")
	}
	oversized := testEvent("oversized", "security", now)
	oversized.Details = make(map[string]string, 20)
	for index := range 20 {
		oversized.Details[string(rune('a'+index))] = strings.Repeat("x", 4096)
	}
	if validEvent(oversized, false) {
		t.Fatal("oversized encoded event was accepted")
	}
}

func TestLoadAndDigestExactBounds(t *testing.T) {
	if err := validateDataDir(strings.Repeat("x", maxDataDirBytes)); err != nil {
		t.Fatalf("bounded data directory = %v", err)
	}
	for name, key := range map[string]string{
		"invalid hex": strings.Repeat("z", 64),
		"short":       hex.EncodeToString(bytes.Repeat([]byte{1}, 31)),
	} {
		directory := t.TempDir()
		data, _ := json.Marshal(map[string]string{"key": key})
		if err := os.WriteFile(filepath.Join(directory, "audit_key.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readAuditKey(directory); err == nil {
			t.Fatalf("%s key was accepted", name)
		}
	}
	valid := hex.EncodeToString(bytes.Repeat([]byte{1}, 32))
	if !validDigest(valid) || validDigest(hex.EncodeToString(bytes.Repeat([]byte{1}, 31))) || validDigest(strings.Repeat("z", 64)) {
		t.Fatal("digest boundaries changed")
	}
}

func TestJournalDecoderExactLineCountAndBlankContinuation(t *testing.T) {
	event := testEvent("event", "security", time.Now().UTC())
	line, _ := json.Marshal(event)
	padded := append(append([]byte{}, line...), bytes.Repeat([]byte{' '}, maxAuditEventBytes-len(line))...)
	journal := &Journal{}
	if events, mode, err := journal.decodeJournal(padded); err != nil || len(events) != 1 || mode != "legacy" {
		t.Fatalf("bounded line = %#v, %q, %v", events, mode, err)
	}
	if _, _, err := journal.decodeJournalLine(line, "", "", maxActivityEvents-1); err != nil {
		t.Fatalf("bounded event count = %v", err)
	}
	if _, _, err := journal.decodeJournalLine(padded, "", "", 0); err != nil {
		t.Fatalf("bounded line decode = %v", err)
	}
	twoLines := append(append(append(append([]byte{}, line...), '\n', ' ', '\n'), line...), '\n')
	if events, _, err := journal.decodeJournal(twoLines); err != nil || len(events) != 2 {
		t.Fatalf("journal after blank line = %#v, %v", events, err)
	}
}

func TestSignedEventPredicatesAreIndependent(t *testing.T) {
	journal := &Journal{key: bytes.Repeat([]byte{1}, 32)}
	event := testEvent("event", "security", time.Now().UTC())
	event.Integrity = journal.sign(event)
	if !journal.validSignedEvent(event, "") {
		t.Fatal("valid signed event was rejected")
	}
	invalidDigest := event
	invalidDigest.Integrity = strings.Repeat("z", 64)
	wrongPrevious := event
	wrongPrevious.Previous = strings.Repeat("0", 64)
	for name, test := range map[string]struct {
		journal *Journal
		event   Event
	}{
		"key":      {journal: &Journal{key: bytes.Repeat([]byte{1}, 31)}, event: event},
		"digest":   {journal: journal, event: invalidDigest},
		"previous": {journal: journal, event: wrongPrevious},
	} {
		if test.journal.validSignedEvent(test.event, "") {
			t.Fatalf("invalid signed event %s was accepted", name)
		}
	}
}

func TestJSONArrayConsumesEveryValueAndStopsOnError(t *testing.T) {
	valid := json.NewDecoder(strings.NewReader(`[1,2]`))
	_, _ = valid.Token()
	if err := scanJSONArray(valid); err != nil {
		t.Fatal(err)
	}
	if _, err := valid.Token(); err == nil {
		t.Fatal("valid array retained trailing tokens")
	}
	invalid := json.NewDecoder(strings.NewReader(`[1,{"a":1,"a":2}]`))
	_, _ = invalid.Token()
	if err := scanJSONArray(invalid); err == nil {
		t.Fatal("duplicate field after first array value was accepted")
	}
}

func TestStorageExactSizeAndOpenedFilePredicates(t *testing.T) { //nolint:cyclop,funlen,gocognit // One file fixture isolates each opened-journal trust predicate.
	event := testEvent("event", "security", time.Now().UTC())
	encoded, _ := json.Marshal(event)
	lineLength := int64(len(encoded) + 1)
	exactPath := filepath.Join(t.TempDir(), "exact.jsonl")
	exact, err := os.OpenFile(exactPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := exact.Truncate(maxAuditJournalBytes - lineLength); err != nil {
		t.Fatal(err)
	}
	_ = exact.Close()
	if err := appendAuditEvent(exactPath, event); err != nil {
		t.Fatalf("exact journal size = %v", err)
	}
	if info, err := os.Stat(exactPath); err != nil || info.Size() != maxAuditJournalBytes {
		t.Fatalf("exact journal info = %#v, %v", info, err)
	}
	if !journalLineFits(maxAuditJournalBytes-11, 10) || journalLineFits(maxAuditJournalBytes-10, 10) {
		t.Fatal("journal line size boundary changed")
	}
	directory := t.TempDir()
	openedFile, err := os.Create(filepath.Join(directory, "opened"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = openedFile.Close() })
	if err := openedFile.Chmod(0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := openedFile.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !validOpenedJournal(nil, opened, false) {
		t.Fatal("new regular journal was rejected")
	}
	if appendableJournal(nil, opened, false, os.ErrInvalid, 1) || !appendableJournal(nil, opened, false, nil, 1) {
		t.Fatal("opened journal stat validation changed")
	}
	otherFile, err := os.Create(filepath.Join(directory, "other"))
	if err != nil {
		t.Fatal(err)
	}
	defer otherFile.Close()
	if err := otherFile.Chmod(0o600); err != nil {
		t.Fatal(err)
	}
	other, _ := otherFile.Stat()
	if validOpenedJournal(other, opened, true) {
		t.Fatal("replaced existing journal was accepted")
	}
	directoryFile, err := os.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer directoryFile.Close()
	directoryInfo, _ := directoryFile.Stat()
	if validOpenedJournal(directoryInfo, opened, true) || appendableJournal(directoryInfo, opened, true, nil, 1) {
		t.Fatal("non-regular linked journal was accepted")
	}
	if runtime.GOOS != "windows" {
		insecurePath := filepath.Join(directory, "insecure")
		if err := os.WriteFile(insecurePath, nil, 0o666); err != nil { //nolint:gosec // The test deliberately creates insecure permissions.
			t.Fatal(err)
		}
		insecure, _ := os.Stat(insecurePath)
		if validOpenedJournal(insecure, insecure, true) || validOpenedJournal(insecure, opened, true) {
			t.Fatal("insecure existing journal was accepted")
		}
	}
}

func TestPruneKeepsExactRetentionBoundary(t *testing.T) {
	now := time.Now().UTC()
	exact := testEvent("exact", "security", now.Add(-time.Hour))
	old := testEvent("old", "security", now.Add(-time.Hour-time.Nanosecond))
	journal := &Journal{events: []Event{exact, old}, auditRetention: time.Hour}
	if !journal.pruneLocked(now) || len(journal.events) != 1 || journal.events[0].ID != "exact" {
		t.Fatalf("retention boundary = %#v", journal.events)
	}
	exactCount := make([]Event, maxActivityEvents)
	for index := range exactCount {
		exactCount[index] = testEvent("event", "security", now)
	}
	journal = &Journal{events: exactCount, auditRetention: time.Hour}
	if journal.pruneLocked(now) || len(journal.events) != maxActivityEvents {
		t.Fatalf("exact activity count was pruned: %d", len(journal.events))
	}
}
