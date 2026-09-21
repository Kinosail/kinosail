package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	subtitleLedgerVersion       = 2
	subtitleLedgerLimit         = 100000
	subtitleLedgerSizeLimit     = 4 << 20
	subDLAutomaticDownloadLimit = 40
)

type subtitleRecord struct {
	BackupRole                string   `json:"backup_role,omitempty"`
	BackupFingerprint         string   `json:"backup_fingerprint,omitempty"`
	Role                      string   `json:"role,omitempty"`
	BackupOriginalFingerprint string   `json:"backup_original_fingerprint,omitempty"`
	OriginalFingerprint       string   `json:"original_fingerprint,omitempty"`
	Fingerprint               string   `json:"fingerprint"`
	Source                    string   `json:"source"`
	Score                     int      `json:"score"`
	CheckedAt                 int64    `json:"checked_at"`
	Managed                   bool     `json:"managed"`
	Size                      int64    `json:"size,omitempty"`
	Modified                  int64    `json:"modified,omitempty"`
	ReleaseMatch              float64  `json:"release_match,omitempty"`
	InstalledAt               int64    `json:"installed_at,omitempty"`
	Cleanup                   []string `json:"cleanup,omitempty"`
	TimingEvidence            string   `json:"timing_evidence,omitempty"`
	Synchronization           string   `json:"synchronization,omitempty"`
	Frozen                    bool     `json:"frozen,omitempty"`
	Backup                    bool     `json:"backup,omitempty"`
}

type subtitleSearchRecord struct {
	Outcome   string `json:"outcome"`
	Attempts  int    `json:"attempts"`
	CheckedAt int64  `json:"checked_at"`
	NextAt    int64  `json:"next_at"`
	SafeError string `json:"safe_error,omitempty"`
}

type subtitleLedgerState struct {
	Version        int                             `json:"version"`
	Records        map[string]subtitleRecord       `json:"records"`
	Searches       map[string]subtitleSearchRecord `json:"searches,omitempty"`
	SubDLDay       string                          `json:"subdl_day,omitempty"`
	SubDLDownloads int                             `json:"subdl_downloads,omitempty"`
}

type subtitleLedger struct {
	mu    sync.Mutex
	path  string
	state subtitleLedgerState
	err   error
}

func newSubtitleLedger(dataDir string) *subtitleLedger {
	ledger := &subtitleLedger{state: subtitleLedgerState{Version: subtitleLedgerVersion, Records: make(map[string]subtitleRecord), Searches: make(map[string]subtitleSearchRecord)}}
	if dataDir == "" {
		return ledger
	}
	ledger.path = filepath.Join(dataDir, "subtitle_acquisitions.json")
	ledger.err = ledger.load()
	return ledger
}

func (ledger *subtitleLedger) load() error { //nolint:cyclop // One bounded loader validates and migrates both supported ledger versions.
	info, err := os.Stat(ledger.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || info.IsDir() || info.Size() > subtitleLedgerSizeLimit {
		return errors.New("subtitle acquisition state is invalid")
	}
	file, err := os.Open(ledger.path) //nolint:gosec // The path is fixed installation-owned state.
	if err != nil {
		return errors.New("subtitle acquisition state is unavailable")
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, subtitleLedgerSizeLimit+1))
	decoder.DisallowUnknownFields()
	var state subtitleLedgerState
	if decoder.Decode(&state) != nil || decoder.Decode(&struct{}{}) != io.EOF || !validSubtitleLedgerState(state) {
		return errors.New("subtitle acquisition state is invalid")
	}
	if state.Records == nil {
		state.Records = make(map[string]subtitleRecord)
	}
	if state.Searches == nil {
		state.Searches = make(map[string]subtitleSearchRecord)
	}
	state.Version = subtitleLedgerVersion
	ledger.state = state
	return nil
}

func validSubtitleLedgerState(state subtitleLedgerState) bool { //nolint:cyclop // Persisted state requires explicit validation of every map key and record.
	if state.Version != 1 && state.Version != subtitleLedgerVersion || len(state.Records) > subtitleLedgerLimit || len(state.Searches) > subtitleLedgerLimit || state.SubDLDownloads < 0 || state.SubDLDownloads > subDLAutomaticDownloadLimit {
		return false
	}
	for key, search := range state.Searches {
		parts := strings.Split(key, ":")
		if len(parts) != 3 || !validSubtitleItemID(parts[0]) || !validLanguage(parts[1]) || !oneOf(parts[2], "standard", "sdh", "forced") || !validSubtitleSearchRecord(search) {
			return false
		}
	}
	for key, record := range state.Records {
		parts := strings.Split(key, ":")
		if len(parts) != 2 || !validSubtitleItemID(parts[0]) || !validLanguage(parts[1]) || !validSubtitleRecord(record) {
			return false
		}
	}
	return state.SubDLDay == "" && state.SubDLDownloads == 0 || validSubtitleDay(state.SubDLDay)
}

func validSubtitleRecord(record subtitleRecord) bool {
	if !validSubtitleRecordKinds(record) || !validSubtitleRecordBounds(record) || !validSubtitleRecordTimes(record) {
		return false
	}
	for _, fingerprint := range []string{record.OriginalFingerprint, record.BackupOriginalFingerprint, record.BackupFingerprint} {
		if fingerprint != "" && !validSubtitleFingerprint(fingerprint) {
			return false
		}
	}
	decoded, err := hex.DecodeString(record.Fingerprint)
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(record.Fingerprint) == record.Fingerprint
}

func validSubtitleRecordKinds(record subtitleRecord) bool {
	return oneOf(record.BackupRole, "", "translation", "captions") && oneOf(record.Role, "", "translation", "captions") && oneOf(record.TimingEvidence, "", "unverified", "audio", "file-hash", "embedded", "manual") && oneOf(record.Synchronization, "", "none", "global", "linear", "piecewise", "manual") && oneOf(record.Source, "embedded", "subdl", "opensubtitles", "subsource", "external", "ocr", "transcription")
}

func validSubtitleRecordBounds(record subtitleRecord) bool {
	return len(record.Fingerprint) == 64 && !(record.Score < 0 || record.Score > 100 || record.ReleaseMatch < 0 || record.ReleaseMatch > 1) && record.Size >= 0 && record.Size <= 4<<20 && len(record.Cleanup) <= 8 && len(strings.Join(record.Cleanup, "")) <= 512
}

func validSubtitleRecordTimes(record subtitleRecord) bool {
	return record.CheckedAt >= 0 && record.CheckedAt <= time.Now().Add(24*time.Hour).Unix() && record.InstalledAt >= 0 && record.InstalledAt <= time.Now().Add(24*time.Hour).Unix() && record.Modified >= 0
}

func validSubtitleSearchRecord(record subtitleSearchRecord) bool {
	return oneOf(record.Outcome, "installed", "no-result", "provider-error") && record.Attempts >= 0 && record.Attempts <= 1000 && record.CheckedAt >= 0 && record.CheckedAt <= time.Now().Add(24*time.Hour).Unix() && record.NextAt >= record.CheckedAt && record.NextAt <= time.Now().Add(8*24*time.Hour).Unix() && len(record.SafeError) <= 256
}

func validSubtitleDay(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func subtitleRecordKey(itemID, language string) string { return itemID + ":" + language }

func subtitleSearchKey(itemID, language, role string) string {
	return itemID + ":" + language + ":" + role
}

func subtitleFingerprint(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func (ledger *subtitleLedger) record(key string) (subtitleRecord, bool, error) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	record, found := ledger.state.Records[key]
	return record, found, ledger.err
}

func (ledger *subtitleLedger) store(key string, record subtitleRecord) error { //nolint:dupl // Separate record map types need the same durable rollback sequence.
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	available := ledger.err == nil && validSubtitleRecord(record) && (len(ledger.state.Records) < subtitleLedgerLimit || ledger.state.Records[key].Fingerprint != "")
	return storeLedgerRecord(ledger.state.Records, key, record, available, ledger.save)
}

func (ledger *subtitleLedger) search(key string) (subtitleSearchRecord, bool, error) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	record, found := ledger.state.Searches[key]
	return record, found, ledger.err
}

func (ledger *subtitleLedger) storeSearch(key string, record subtitleSearchRecord) error { //nolint:dupl // Separate record map types need the same durable rollback sequence.
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	available := ledger.err == nil && validSubtitleSearchRecord(record) && (len(ledger.state.Searches) < subtitleLedgerLimit || ledger.state.Searches[key].Outcome != "")
	return storeLedgerRecord(ledger.state.Searches, key, record, available, ledger.save)
}

func storeLedgerRecord[T any](records map[string]T, key string, record T, available bool, save func() error) error {
	if !available {
		return errors.New("subtitle acquisition state is unavailable")
	}
	previous, found := records[key]
	records[key] = record
	if err := save(); err != nil {
		if found {
			records[key] = previous
		} else {
			delete(records, key)
		}
		return err
	}
	return nil
}

func (ledger *subtitleLedger) automaticSearchReady(key string, now time.Time) bool {
	record, found, err := ledger.search(key)
	return err == nil && (!found || now.Unix() >= record.NextAt)
}

func (ledger *subtitleLedger) noteSearch(key, outcome, safeError string, now time.Time) {
	previous, found, _ := ledger.search(key)
	attempts := 0
	if found && outcome != "installed" {
		attempts = min(previous.Attempts+1, 1000)
	}
	delay := 24 * time.Hour
	switch outcome {
	case "provider-error":
		delay = min(time.Duration(1<<min(attempts, 6))*time.Minute, time.Hour)
	case "no-result":
		delays := []time.Duration{6 * time.Hour, 24 * time.Hour, 72 * time.Hour, 7 * 24 * time.Hour}
		delay = delays[min(attempts, len(delays)-1)]
	}
	_ = ledger.storeSearch(key, subtitleSearchRecord{Outcome: outcome, Attempts: attempts, CheckedAt: now.Unix(), NextAt: now.Add(delay).Unix(), SafeError: safeError})
}

func (ledger *subtitleLedger) lastSuccessfulWrite() time.Time {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	latest := int64(0)
	for _, record := range ledger.state.Records {
		latest = max(latest, record.InstalledAt)
	}
	return time.Unix(latest, 0)
}

func (ledger *subtitleLedger) takeSubDLDownload(now time.Time) bool {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.err != nil {
		return false
	}
	day := now.UTC().Format("2006-01-02")
	previousDay, previousCount := ledger.state.SubDLDay, ledger.state.SubDLDownloads
	if ledger.state.SubDLDay != day {
		ledger.state.SubDLDay, ledger.state.SubDLDownloads = day, 0
	}
	if ledger.state.SubDLDownloads >= subDLAutomaticDownloadLimit {
		return false
	}
	ledger.state.SubDLDownloads++
	if err := ledger.save(); err != nil {
		ledger.state.SubDLDay, ledger.state.SubDLDownloads = previousDay, previousCount
		return false
	}
	return true
}

func (ledger *subtitleLedger) save() error {
	if ledger.path == "" {
		return nil
	}
	return saveDurableJSON(ledger.path, ledger.state)
}
