package auditjournal

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type auditErrorWriter struct{}

func (auditErrorWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestJournalValidationAndViewFailureBranches(t *testing.T) { //nolint:cyclop // Exact validation assertions cover independent trust-boundary failures.
	journal := New(t.Context(), Config{})
	if journal.TripwireWarning() != "" {
		t.Fatal("empty journal reported a tripwire warning")
	}
	journal.Record(testEvent("event", "security", time.Now().UTC()))
	if err := journal.WriteJSONL(auditErrorWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("JSONL writer error = %v", err)
	}
	chained := testEvent("chained", "security", time.Now().UTC())
	chained.Previous = strings.Repeat("0", 64)
	if validEvent(chained, false) {
		t.Fatal("new event accepted stored integrity fields")
	}
	optional := testEvent("optional", "security", time.Now().UTC())
	optional.Actor = "invalid\x00actor"
	if validEvent(optional, false) {
		t.Fatal("event accepted an invalid optional field")
	}
	details := testEvent("details", "security", time.Now().UTC())
	details.Details = make(map[string]string, 129)
	for index := range 129 {
		details.Details[string(rune(index+1))] = "value"
	}
	if validEvent(details, false) {
		t.Fatal("event accepted too many detail fields")
	}
}

func TestJournalDecoderFailureBranches(t *testing.T) { //nolint:cyclop,funlen // Direct decoder states cover otherwise malformed persisted inputs.
	journal := &Journal{}
	line, err := json.Marshal(testEvent("event", "security", time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := journal.decodeJournalLine(line, "", "", maxActivityEvents); err == nil {
		t.Fatal("maximum event count was accepted")
	}
	legacy := testEvent("legacy", "security", time.Now().UTC())
	legacy.Previous = strings.Repeat("0", 64)
	if _, _, err := validateLegacyEvent(legacy); err == nil {
		t.Fatal("legacy previous digest was accepted")
	}
	if journal.validSignedEvent(Event{Integrity: strings.Repeat("0", 64)}, "") {
		t.Fatal("signed event was accepted without a key")
	}
	var target map[string]any
	if err := decodeStrictJSON([]byte(`{} {}`), &target); err == nil {
		t.Fatal("strict decoder accepted trailing JSON")
	}
	if err := rejectDuplicateJSONFields([]byte(`true false`)); err == nil {
		t.Fatal("field scanner accepted trailing JSON")
	}
	unexpected := json.NewDecoder(strings.NewReader(`{"field":1}`))
	_, _ = unexpected.Token()
	_, _ = unexpected.Token()
	_, _ = unexpected.Token()
	if err := scanJSONValue(unexpected, false); err == nil {
		t.Fatal("unexpected delimiter was accepted")
	}
	invalidObject := json.NewDecoder(strings.NewReader(`{"field":1}`))
	_, _ = invalidObject.Token()
	_, _ = invalidObject.Token()
	if err := scanJSONObject(invalidObject, false); err == nil {
		t.Fatal("invalid object field token was accepted")
	}
	brokenObject := json.NewDecoder(strings.NewReader(`{!}`))
	_, _ = brokenObject.Token()
	if err := scanJSONObject(brokenObject, false); err == nil {
		t.Fatal("truncated object field was accepted")
	}
	for name, input := range map[string]string{"object value": `{"field":]}`, "array value": `[}]`} {
		if err := rejectDuplicateJSONFields([]byte(input)); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	if err := rejectDuplicateJSONFields([]byte(`[`)); err == nil {
		t.Fatal("incomplete array was accepted")
	}
	wrongDelimiter := json.NewDecoder(strings.NewReader(`[]`))
	_, _ = wrongDelimiter.Token()
	if err := consumeJSONDelimiter(wrongDelimiter, '}'); err == nil {
		t.Fatal("wrong closing delimiter was accepted")
	}
	if err := consumeJSONDelimiter(json.NewDecoder(bytes.NewReader(nil)), ']'); err == nil {
		t.Fatal("missing closing delimiter was accepted")
	}
}

func TestJournalKeyAndLoadFailureBranches(t *testing.T) { //nolint:cyclop // Storage failures prove every load stage fails closed.
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "audit_key.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readAuditKey(directory); err == nil {
		t.Fatal("directory key was accepted")
	}
	journal := &Journal{file: directory}
	if _, err := journal.loadJournal(); err == nil {
		t.Fatal("directory journal was accepted")
	}
	originalRandom := rand.Reader
	rand.Reader = auditErrorReader{}
	t.Cleanup(func() { rand.Reader = originalRandom })
	if err := (&Journal{}).createAuditKey(t.TempDir()); err == nil {
		t.Fatal("randomness failure was accepted")
	}
	if err := (&Journal{}).load(t.TempDir()); err == nil {
		t.Fatal("load accepted randomness failure")
	}
	rand.Reader = originalRandom
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (&Journal{}).createAuditKey(blocked); err == nil {
		t.Fatal("blocked key path was accepted")
	}
	if runtime.GOOS != "windows" {
		testRewriteFailureDuringLoad(t)
	}
}

type auditErrorReader struct{}

func (auditErrorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func testRewriteFailureDuringLoad(t *testing.T) {
	t.Helper()
	directory := t.TempDir()
	first := New(t.Context(), Config{DataDir: directory})
	if !first.Status().Healthy {
		t.Fatal("could not create initial journal key")
	}
	legacy, _ := json.Marshal(testEvent("legacy", "security", time.Now().UTC()))
	if err := os.WriteFile(filepath.Join(directory, "audit.jsonl"), append(legacy, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o500); err != nil { //nolint:gosec // The test deliberately removes directory write permission.
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o700) }) //nolint:gosec // Cleanup restores private directory access.
	if err := (&Journal{}).load(directory); err == nil {
		t.Fatal("load accepted an unwritable legacy journal")
	}
}

func TestJournalStorageFailureBranches(t *testing.T) { //nolint:cyclop,funlen,gocognit // Direct file states cover rollback and validation branches.
	directory := t.TempDir()
	oversized := testEvent("oversized", "security", time.Now().UTC())
	oversized.Details = map[string]string{"value": strings.Repeat("x", maxAuditEventBytes)}
	if err := appendAuditEvent(filepath.Join(directory, "oversized.jsonl"), oversized); err == nil {
		t.Fatal("oversized append was accepted")
	}
	if _, _, _, err := openAuditJournal("invalid\x00path"); err == nil {
		t.Fatal("invalid journal path was opened")
	}
	blocked := filepath.Join(directory, "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := appendAuditEvent(blocked, testEvent("event", "security", time.Now().UTC())); err == nil {
		t.Fatal("directory journal was appended")
	}
	large := filepath.Join(directory, "large.jsonl")
	file, err := os.OpenFile(large, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxAuditJournalBytes); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if err := appendAuditEvent(large, testEvent("event", "security", time.Now().UTC())); err == nil {
		t.Fatal("full journal accepted another event")
	}
	created := filepath.Join(directory, "created.jsonl")
	createdFile, err := os.OpenFile(created, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := rejectAuditJournal(created, createdFile, false); err == nil {
		t.Fatal("new invalid journal was accepted")
	}
	if _, err := os.Stat(created); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new invalid journal was retained: %v", err)
	}
	closed, err := os.Create(filepath.Join(directory, "closed.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	_ = closed.Close()
	if err := appendJournalLine(closed.Name(), closed, []byte("line\n"), true, closed.Write); err == nil {
		t.Fatal("closed journal was appended")
	}
	newPath := filepath.Join(directory, "new.jsonl")
	newFile, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := appendJournalLine(newPath, newFile, []byte("line\n"), false, func([]byte) (int, error) { return 0, io.ErrClosedPipe }); err == nil {
		t.Fatal("failed new append was accepted")
	}
	if _, err := os.Stat(newPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed new journal was retained: %v", err)
	}
	shortPath := filepath.Join(directory, "short.jsonl")
	shortFile, err := os.OpenFile(shortPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	line := []byte("line\n")
	if err := appendJournalLine(shortPath, shortFile, line, true, func([]byte) (int, error) { return len(line) - 1, nil }); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write error = %v", err)
	}
	closePath := filepath.Join(directory, "close.jsonl")
	closeFile, err := os.OpenFile(closePath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := appendJournalLineWithClose(closePath, closeFile, line, true, closeFile.Write, func() error { return io.ErrClosedPipe }); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("close error = %v", err)
	}
	_ = closeFile.Close()
	removePath := filepath.Join(directory, "remove-on-close.jsonl")
	removeFile, err := os.OpenFile(removePath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := appendJournalLineWithClose(removePath, removeFile, line, false, removeFile.Write, func() error { return io.ErrClosedPipe }); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("new close error = %v", err)
	}
	_ = removeFile.Close()
	nilQueue := &Journal{}
	nilQueue.enqueueNotification(testEvent("event", "security", time.Now().UTC()))
	if nilQueue.notificationDrops.Load() != 2 {
		t.Fatalf("nil queue drops = %d", nilQueue.notificationDrops.Load())
	}
}

func TestJournalPruneRewriteAndOpenedValidationBranches(t *testing.T) { //nolint:cyclop // Limit and file-shape assertions cover bounded storage helpers.
	events := make([]Event, maxActivityEvents+1)
	for index := range events {
		events[index] = testEvent("event", "security", time.Now().UTC())
	}
	journal := &Journal{events: events, auditRetention: time.Hour}
	if !journal.pruneLocked(time.Now().UTC()) || len(journal.events) != maxActivityEvents {
		t.Fatalf("pruned event count = %d", len(journal.events))
	}
	if err := (&Journal{}).rewriteLocked(); err != nil {
		t.Fatalf("empty rewrite = %v", err)
	}
	journal = &Journal{file: filepath.Join(t.TempDir(), "audit.jsonl"), events: []Event{{Details: map[string]string{"value": strings.Repeat("x", maxAuditJournalBytes)}}}}
	if err := journal.rewriteLocked(); err == nil {
		t.Fatal("oversized rewrite was accepted")
	}
	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	info, err := directory.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if validOpenedJournal(info, info, true) {
		t.Fatal("directory accepted as journal")
	}
}
