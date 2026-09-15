package auditjournal

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

const (
	maxAuditEventBytes   = 65_536
	maxAuditJournalBytes = 67_108_864
	maxAuditKeyBytes     = 4096
	maxDataDirBytes      = 4096
)

func (journal *Journal) load(dataDir string) error {
	if err := validateDataDir(dataDir); err != nil {
		return err
	}
	if dataDir == "" {
		journal.writable = true
		return nil
	}
	journal.file = filepath.Join(dataDir, "audit.jsonl")
	key, keyExists, err := readAuditKey(dataDir)
	if err != nil {
		return err
	}
	journal.key = key
	legacy, err := journal.loadJournal()
	if err != nil {
		return err
	}
	if !keyExists {
		if err := journal.createAuditKey(dataDir); err != nil {
			return err
		}
	}
	if legacy || journal.pruneLocked(time.Now().UTC()) {
		if err := journal.rewriteLocked(); err != nil {
			return err
		}
	}
	journal.writable = true
	return nil
}

func validateDataDir(dataDir string) error {
	if len(dataDir) > maxDataDirBytes || strings.ContainsRune(dataDir, 0) {
		return errors.New("activity journal data directory is invalid")
	}
	return nil
}

func readAuditKey(dataDir string) ([]byte, bool, error) {
	path := filepath.Join(dataDir, "audit_key.json")
	data, err := privatefile.Read(path, maxAuditKeyBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read activity journal key: %w", err)
	}
	var saved struct {
		Key string `json:"key"`
	}
	if decodeStrictJSON(data, &saved) != nil {
		return nil, false, errors.New("activity journal key is invalid")
	}
	key, decodeErr := hex.DecodeString(saved.Key)
	if decodeErr != nil || len(key) != 32 {
		return nil, false, errors.New("activity journal key is invalid")
	}
	return key, true, nil
}

func (journal *Journal) createAuditKey(dataDir string) error {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return fmt.Errorf("create activity journal key: %w", err)
	}
	data, _ := json.Marshal(struct {
		Key string `json:"key"`
	}{hex.EncodeToString(key)})
	if err := privatefile.Write(filepath.Join(dataDir, "audit_key.json"), data); err != nil {
		return fmt.Errorf("save activity journal key: %w", err)
	}
	journal.key = key
	return nil
}

func (journal *Journal) loadJournal() (bool, error) {
	data, err := privatefile.Read(journal.file, maxAuditJournalBytes)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read activity journal: %w", err)
	}
	events, mode, err := journal.decodeJournal(data)
	if err != nil {
		return false, err
	}
	journal.events = events
	return mode == "legacy", nil
}

func (journal *Journal) decodeJournal(data []byte) ([]Event, string, error) {
	events := make([]Event, 0, min(1024, maxActivityEvents))
	previous := ""
	mode := ""
	for _, raw := range bytes.Split(data, []byte{'\n'}) {
		if len(raw) > maxAuditEventBytes {
			return nil, "", errors.New("activity journal exceeds event limits")
		}
		line := bytes.TrimSpace(raw)
		if len(line) == 0 {
			continue
		}
		event, eventMode, decodeErr := journal.decodeJournalLine(line, previous, mode, len(events))
		if decodeErr != nil {
			return nil, "", decodeErr
		}
		mode = eventMode
		events = append(events, event)
		previous = event.Integrity
	}
	return events, mode, nil
}

func (journal *Journal) decodeJournalLine(line []byte, previous, mode string, count int) (Event, string, error) {
	if len(line) > maxAuditEventBytes || count >= maxActivityEvents {
		return Event{}, "", errors.New("activity journal exceeds event limits")
	}
	event, eventMode, err := journal.decodeJournalEvent(line, previous)
	if err != nil {
		return Event{}, "", err
	}
	if mode != "" && eventMode != mode {
		return Event{}, "", errors.New("activity journal mixes signed and legacy events")
	}
	return event, eventMode, nil
}

func (journal *Journal) decodeJournalEvent(line []byte, previous string) (Event, string, error) {
	var event Event
	if err := decodeStrictJSON(line, &event); err != nil || !validEvent(event, true) {
		return Event{}, "", errors.New("activity journal event is invalid")
	}
	if event.Integrity == "" {
		return validateLegacyEvent(event)
	}
	if !journal.validSignedEvent(event, previous) {
		return Event{}, "", errors.New("activity journal integrity check failed")
	}
	return event, "signed", nil
}

func validateLegacyEvent(event Event) (Event, string, error) {
	if event.Previous != "" {
		return Event{}, "", errors.New("activity journal integrity check failed")
	}
	return event, "legacy", nil
}

func (journal *Journal) validSignedEvent(event Event, previous string) bool {
	if len(journal.key) != 32 {
		return false
	}
	if !validDigest(event.Integrity) {
		return false
	}
	if event.Previous != previous {
		return false
	}
	expected, _ := hex.DecodeString(journal.sign(event))
	actual, _ := hex.DecodeString(event.Integrity)
	return hmac.Equal(actual, expected)
}

func (journal *Journal) sign(event Event) string {
	if len(journal.key) == 0 {
		return ""
	}
	event.Integrity = ""
	data, _ := json.Marshal(event)
	mac := hmac.New(sha256.New, journal.key)
	_, _ = mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func decodeStrictJSON(data []byte, target any) error {
	if err := rejectDuplicateJSONFields(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func rejectDuplicateJSONFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(decoder, true); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("JSON has trailing data")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder, foldFields bool) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		return scanJSONObject(decoder, foldFields)
	case '[':
		return scanJSONArray(decoder)
	default:
		return errors.New("JSON has an unexpected delimiter")
	}
}

func scanJSONObject(decoder *json.Decoder, foldFields bool) error {
	fields := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		field, ok := token.(string)
		if err != nil {
			return errors.New("JSON object is invalid")
		}
		if !ok {
			return errors.New("JSON object is invalid")
		}
		if foldFields {
			field = strings.ToLower(field)
		}
		if _, found := fields[field]; found {
			return errors.New("JSON has duplicate fields")
		}
		fields[field] = struct{}{}
		if err := scanJSONValue(decoder, false); err != nil {
			return err
		}
	}
	return consumeJSONDelimiter(decoder, '}')
}

func scanJSONArray(decoder *json.Decoder) error {
	for decoder.More() {
		err := scanJSONValue(decoder, false)
		switch err {
		case nil:
		default:
			return err
		}
	}
	return consumeJSONDelimiter(decoder, ']')
}

func consumeJSONDelimiter(decoder *json.Decoder, wanted json.Delim) error {
	token, err := decoder.Token()
	if err != nil || token != wanted {
		return errors.New("JSON delimiter is invalid")
	}
	return nil
}
