package auditjournal

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestJournalRoundTripDetectsTamperingAndUnsignedSuffix(t *testing.T) {
	for name, tamper := range map[string]func(string) string{
		"changed event": func(data string) string {
			return strings.Replace(data, `"result":"success"`, `"result":"failure"`, 1)
		},
		"unsigned suffix": func(data string) string {
			legacy, _ := json.Marshal(testEvent("suffix", "security", time.Now().UTC()))
			return data + string(legacy) + "\n"
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			journal := New(t.Context(), Config{DataDir: directory})
			journal.Record(testEvent("first", "security", time.Now().UTC()))
			path := filepath.Join(directory, "audit.jsonl")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			changed := []byte(tamper(string(data)))
			if err := os.WriteFile(path, changed, 0o600); err != nil {
				t.Fatal(err)
			}
			reloaded := New(t.Context(), Config{DataDir: directory})
			if status := reloaded.Status(); status.Healthy || len(reloaded.Query("", 100)) != 0 {
				t.Fatalf("tampered journal = %#v, %#v", status, reloaded.Query("", 100))
			}
			current, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(current, changed) {
				t.Fatalf("rejected journal changed: %v", readErr)
			}
		})
	}
}

func TestLegacyJournalMigratesOnceToSignedChain(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "audit.jsonl")
	first, _ := json.Marshal(testEvent("first", "administration", time.Now().UTC().Add(-time.Minute)))
	second, _ := json.Marshal(testEvent("second", "security", time.Now().UTC()))
	legacy := append(append(append([]byte{}, first...), '\n'), second...)
	legacy = append(legacy, '\n')
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	journal := New(t.Context(), Config{DataDir: directory})
	if status := journal.Status(); !status.Healthy {
		t.Fatalf("legacy migration = %#v", status)
	}
	if _, err := os.Stat(filepath.Join(directory, "audit_key.json")); err != nil {
		t.Fatal(err)
	}
	migrated, err := os.ReadFile(path)
	if err != nil || bytes.Equal(migrated, legacy) || !bytes.Contains(migrated, []byte(`"integrity"`)) {
		t.Fatalf("migrated journal = %q, %v", migrated, err)
	}
	reloaded := New(t.Context(), Config{DataDir: directory})
	if events := reloaded.Query("", 10); !reloaded.Status().Healthy || len(events) != 2 || events[0].ID != "second" {
		t.Fatalf("reloaded events = %#v, %#v", events, reloaded.Status())
	}
}

func TestRetentionUsesSeparateAuditAndPlaybackWindows(t *testing.T) {
	directory := t.TempDir()
	journal := New(t.Context(), Config{DataDir: directory, AuditRetention: time.Hour, PlaybackRetention: 30 * time.Minute})
	now := time.Now().UTC()
	journal.Record(testEvent("old-audit", "administration", now.Add(-2*time.Hour)))
	journal.Record(testEvent("old-playback", "playback", now.Add(-time.Hour)))
	journal.Record(testEvent("current", "security", now))
	if events := journal.Query("", 100); len(events) != 1 || events[0].ID != "current" {
		t.Fatalf("retained events = %#v", events)
	}
	data, err := os.ReadFile(filepath.Join(directory, "audit.jsonl"))
	if err != nil || bytes.Contains(data, []byte("old-audit")) || bytes.Contains(data, []byte("old-playback")) {
		t.Fatalf("retained journal = %q, %v", data, err)
	}
}

func TestInvalidPersistedStateHasNoSideEffects(t *testing.T) {
	for name, data := range map[string][]byte{
		"malformed":             []byte("not-json\n"),
		"unknown":               []byte(`{"id":"event","unknown":true}` + "\n"),
		"duplicate":             []byte(`{"id":"first","id":"second"}` + "\n"),
		"case-folded duplicate": []byte(`{"id":"first","ID":"second"}` + "\n"),
		"wrong nested type":     []byte(`{"id":"event","time":"2026-08-23T00:00:00Z","category":"security","action":"activity.recorded","result":"success","details":[]}` + "\n"),
		"oversized value":       []byte(`{"id":"` + strings.Repeat("x", maxAuditEventBytes) + `"}`),
		"oversized whitespace":  append([]byte(strings.Repeat(" ", maxAuditEventBytes)), []byte("{}")...),
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "audit.jsonl")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			journal := New(t.Context(), Config{DataDir: directory})
			if journal.Status().Healthy || len(journal.Query("", 100)) != 0 {
				t.Fatal("invalid journal was accepted")
			}
			if _, err := os.Lstat(filepath.Join(directory, "audit_key.json")); !os.IsNotExist(err) {
				t.Fatalf("invalid journal created a key: %v", err)
			}
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, data) {
				t.Fatalf("invalid journal changed: %v", err)
			}
		})
	}
}

func TestInvalidConfigurationHasNoSideEffects(t *testing.T) {
	for name, dataDir := range map[string]string{
		"nul":       filepath.Join(t.TempDir(), "invalid\x00directory"),
		"oversized": strings.Repeat("x", maxDataDirBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			journal := New(t.Context(), Config{DataDir: dataDir})
			if status := journal.Status(); status.Healthy || status.WriteFailures != 1 {
				t.Fatalf("invalid configuration = %#v", status)
			}
			if journal.file != "" || len(journal.Query("", 10)) != 0 {
				t.Fatal("invalid configuration changed journal state")
			}
		})
	}
}

func TestInvalidKeyAndEventHaveNoSideEffects(t *testing.T) {
	for name, invalidKey := range map[string][]byte{
		"malformed": []byte(`{"key":"invalid"}`),
		"duplicate": []byte(`{"key":"` + strings.Repeat("0", 64) + `","key":"` + strings.Repeat("1", 64) + `"}`),
	} {
		t.Run(name+" key", func(t *testing.T) {
			directory := t.TempDir()
			keyPath := filepath.Join(directory, "audit_key.json")
			if err := os.WriteFile(keyPath, invalidKey, 0o600); err != nil {
				t.Fatal(err)
			}
			journal := New(t.Context(), Config{DataDir: directory})
			journal.Record(testEvent("blocked", "security", time.Now().UTC()))
			current, err := os.ReadFile(keyPath)
			if err != nil || !bytes.Equal(current, invalidKey) {
				t.Fatalf("invalid key changed: %q, %v", current, err)
			}
			if _, err := os.Lstat(filepath.Join(directory, "audit.jsonl")); !os.IsNotExist(err) {
				t.Fatalf("invalid key created a journal: %v", err)
			}
		})
	}

	validDir := t.TempDir()
	valid := New(t.Context(), Config{DataDir: validDir})
	journalPath := filepath.Join(validDir, "audit.jsonl")
	before, _ := os.ReadFile(journalPath)
	valid.Record(Event{ID: "missing-required-fields"})
	valid.Record(testEvent("invalid\x00id", "security", time.Now().UTC()))
	valid.Record(testEvent("oversized", "security", time.Now().UTC(), strings.Repeat("x", maxAuditEventBytes)))
	after, _ := os.ReadFile(journalPath)
	if !bytes.Equal(before, after) || len(valid.Query("", 10)) != 0 || valid.Status().WriteFailures != 3 {
		t.Fatalf("invalid events changed journal: %#v", valid.Status())
	}
}

func TestPartialAppendRollsBackFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	prefix := []byte("existing\n")
	if err := os.WriteFile(path, prefix, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	line := []byte("partial-event\n")
	err = appendJournalLine(path, file, line, true, func(data []byte) (int, error) {
		written, _ := file.Write(data[:len(data)/2])
		return written, io.ErrShortWrite
	})
	current, readErr := os.ReadFile(path)
	if err == nil || readErr != nil || !bytes.Equal(current, prefix) {
		t.Fatalf("partial append = %q, %v, %v", current, err, readErr)
	}
}

func TestFailedAppendRollsBackMemory(t *testing.T) {
	directory := t.TempDir()
	journal := New(t.Context(), Config{DataDir: directory})
	journal.Record(testEvent("preserved", "security", time.Now().UTC()))
	path := filepath.Join(directory, "audit.jsonl")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	journal.Record(testEvent("rejected", "security", time.Now().UTC()))
	events := journal.Query("", 10)
	if status := journal.Status(); status.Healthy || status.WriteFailures != 1 || len(events) != 1 || events[0].ID != "preserved" {
		t.Fatalf("failed append = %#v, %#v", status, events)
	}
}

func TestNotificationsKeepBoundedLatestQueue(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	delivered := make(chan string, auditNotificationQueue+2)
	journal := New(ctx, Config{Notify: blockingNotifier(started, release, delivered)})
	journal.Record(testEvent("first", "security", time.Now().UTC()))
	waitForSignal(t, started)
	for index := range auditNotificationQueue + 2 {
		journal.Record(testEvent(strconv.Itoa(index), "security", time.Now().UTC()))
	}
	if journal.Status().NotificationDrops == 0 {
		t.Fatal("overflow was not reported")
	}
	close(release)
	waitForNotification(t, delivered, strconv.Itoa(auditNotificationQueue+1))
}

func blockingNotifier(started chan struct{}, release <-chan struct{}, delivered chan<- string) func(Event) {
	return func(event Event) {
		select {
		case <-started:
		default:
			close(started)
			<-release
		}
		delivered <- event.ID
	}
}

func waitForSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("notification worker did not start")
	}
}

func waitForNotification(t *testing.T, delivered <-chan string, wanted string) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case id := <-delivered:
			if id == wanted {
				return
			}
		case <-deadline:
			t.Fatal("latest notification was not retained")
		}
	}
}

func testEvent(id, category string, created time.Time, details ...string) Event {
	event := Event{ID: id, Time: created.Format(time.RFC3339Nano), Category: category, Action: "activity.recorded", Result: "success"}
	if len(details) != 0 {
		event.Details = map[string]string{"value": details[0]}
	}
	return event
}
