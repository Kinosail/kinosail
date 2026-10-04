package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const subtitleOperationStoreLimit = 96 * 1024

type subtitleOperationReceipt struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Item    string `json:"item,omitempty"`
	State   string `json:"state"`
	Outcome string `json:"outcome,omitempty"`
	Status  int    `json:"status,omitempty"`
}

type subtitleOperationRecord struct {
	subtitleOperationReceipt
	Owner     string `json:"owner"`
	Digest    string `json:"digest,omitempty"`
	Created   int64  `json:"created"`
	Expires   int64  `json:"expires"`
	Deadline  int64  `json:"deadline,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

type subtitleOperationEnvelope struct {
	Version  int                       `json:"version"`
	Receipts []subtitleOperationRecord `json:"receipts"`
}

func (operations *subtitleOperations) load(data string) {
	if data == "" {
		return
	}
	operations.path = filepath.Join(data, "subtitle_operations.json")
	envelope, err := readSubtitleOperationEnvelope(data)
	if errors.Is(err, os.ErrNotExist) {
		operations.available = true
		return
	}
	if err != nil || !validSubtitleOperationEnvelope(envelope) {
		return
	}
	operations.restore(envelope)
	operations.available = operations.persist() == nil
}

func readSubtitleOperationEnvelope(data string) (subtitleOperationEnvelope, error) {
	var envelope subtitleOperationEnvelope
	root, err := os.OpenRoot(data)
	if err != nil {
		return envelope, err
	}
	defer root.Close()
	info, err := root.Lstat("subtitle_operations.json")
	if err != nil {
		return envelope, err
	}
	if !info.Mode().IsRegular() || info.Size() > subtitleOperationStoreLimit || info.Mode().Perm()&0o077 != 0 {
		return envelope, errors.New("receipt metadata is unavailable")
	}
	file, err := root.Open("subtitle_operations.json")
	if err != nil {
		return envelope, err
	}
	defer file.Close()
	err = httpguard.DecodeUniqueJSON(file, subtitleOperationStoreLimit, &envelope)
	return envelope, err
}

func validSubtitleOperationEnvelope(envelope subtitleOperationEnvelope) bool {
	if envelope.Version != 1 || envelope.Receipts == nil || len(envelope.Receipts) > 64 {
		return false
	}
	seen := map[string]bool{}
	for _, record := range envelope.Receipts {
		if !validSubtitleOperationRecord(record) || seen[record.ID] {
			return false
		}
		seen[record.ID] = true
	}
	return true
}

func (operations *subtitleOperations) restore(envelope subtitleOperationEnvelope) {
	now := operations.clock.Now().Unix()
	for _, record := range envelope.Receipts {
		// A failed durable activation must never resurrect preparation. An
		// interrupted running record is retained as unknown, never resumed.
		if record.State == "prepared" || record.State != "running" && record.Expires <= now {
			continue
		}
		if record.State == "running" {
			record.State, record.Outcome, record.Status = "unknown", "uncertain", 0
			record.Completed, record.Expires = now, now+int64((30*time.Minute)/time.Second)
		}
		operations.records[record.ID] = record
	}
}

func validSubtitleOperationRecord(record subtitleOperationRecord) bool {
	if !validSubtitleFingerprint(record.ID) || !validSubtitleOperationInput(record.Action, record.Item) || !validSubtitleOperationOwner(record.Owner) || record.Created <= 0 || record.Expires < 0 {
		return false
	}
	switch record.State {
	case "prepared":
		return validPreparedSubtitleOperation(record)
	case "running":
		return validRunningSubtitleOperation(record)
	case "completed":
		return validCompletedSubtitleOperation(record)
	case "unknown":
		return validUnknownSubtitleOperation(record)
	default:
		return false
	}
}

func validSubtitleOperationOwner(owner string) bool {
	return owner != "" && len(owner) <= 128 && strings.Trim(owner, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") == ""
}

func validPreparedSubtitleOperation(record subtitleOperationRecord) bool {
	return record.Digest == "" && record.Status == 0 && record.Outcome == "" && record.Expires > record.Created && record.Completed == 0 && record.Deadline == 0
}

func validRunningSubtitleOperation(record subtitleOperationRecord) bool {
	return validSubtitleFingerprint(record.Digest) && record.Deadline > record.Created && record.Completed == 0 && record.Status == 0 && record.Outcome == "" && record.Expires == 0
}

func validCompletedSubtitleOperation(record subtitleOperationRecord) bool {
	return validSubtitleFingerprint(record.Digest) && record.Completed >= record.Created && record.Expires > record.Completed && record.Status >= 200 && record.Status <= 599 && oneOf(record.Outcome, "success", "failed")
}

func validUnknownSubtitleOperation(record subtitleOperationRecord) bool {
	return validSubtitleFingerprint(record.Digest) && record.Completed >= record.Created && record.Expires > record.Completed && record.Status == 0 && record.Outcome == "uncertain"
}

func (operations *subtitleOperations) persist() error {
	envelope := subtitleOperationEnvelope{Version: 1, Receipts: make([]subtitleOperationRecord, 0, len(operations.records))}
	for _, record := range operations.records {
		envelope.Receipts = append(envelope.Receipts, record)
	}
	sort.Slice(envelope.Receipts, func(i, j int) bool { return envelope.Receipts[i].ID < envelope.Receipts[j].ID })
	data, err := json.Marshal(envelope)
	if err != nil || len(data) > subtitleOperationStoreLimit || operations.path == "" {
		return errors.New("receipt metadata is unavailable")
	}
	return writeSubtitleOperationStore(operations.path, data)
}

func writeSubtitleOperationStore(path string, data []byte) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	name := ".subtitle-operations-" + rand.Text()
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(name) }()
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = root.Rename(name, "subtitle_operations.json"); err != nil {
		return err
	}
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (operations *subtitleOperations) expire() {
	now := operations.clock.Now().Unix()
	for id, record := range operations.records {
		if record.State != "running" && record.Expires <= now {
			delete(operations.records, id)
			delete(operations.results, id)
		}
	}
}

func subtitleOperationBody(requestBody io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(requestBody, (10<<20)+1))
	if err != nil || len(data) > 10<<20 {
		return nil, errors.New("subtitle operation body is invalid")
	}
	return data, nil
}
