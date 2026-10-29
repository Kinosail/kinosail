package server

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
)

const (
	subtitleLedgerVersion       = 4
	subtitleLedgerLimit         = 100000
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
	Outcome       string `json:"outcome"`
	Attempts      int    `json:"attempts"`
	CheckedAt     int64  `json:"checked_at"`
	NextAt        int64  `json:"next_at"`
	SafeError     string `json:"safe_error,omitempty"`
	MediaSize     int64  `json:"media_size,omitempty"`
	MediaModified int64  `json:"media_modified,omitempty"`
}

type subtitleLedgerState struct {
	Version        int                             `json:"version"`
	Records        map[string]subtitleRecord       `json:"records"`
	Searches       map[string]subtitleSearchRecord `json:"searches,omitempty"`
	History        []subtitleHistoryEvent          `json:"history,omitempty"`
	SubDLDay       string                          `json:"subdl_day,omitempty"`
	SubDLDownloads int                             `json:"subdl_downloads,omitempty"`
}

type subtitleLedger struct {
	mu      sync.Mutex
	path    string
	db      *database.Store
	state   subtitleLedgerState
	err     error
	publish func()
	missing map[string]time.Time
}

func (ledger *subtitleLedger) setPublisher(publish func()) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	ledger.publish = publish
}

func (ledger *subtitleLedger) publishChange() {
	ledger.mu.Lock()
	publish := ledger.publish
	ledger.mu.Unlock()
	if publish != nil {
		publish()
	}
}

func newSubtitleLedger(dataDir string, databases ...*database.Store) *subtitleLedger {
	ledger := &subtitleLedger{state: emptySubtitleLedgerState()}
	if dataDir == "" {
		return ledger
	}
	ledger.path = filepath.Join(dataDir, subtitleLedgerLegacyFile)
	ledger.db = configuredDatabase(databases)
	if ledger.db != nil {
		ledger.err = ledger.open()
	}
	return ledger
}

func validSubtitleLedgerState(state subtitleLedgerState) bool {
	if !validSubtitleLedgerBounds(state) {
		return false
	}
	for _, event := range state.History {
		if !validSubtitleHistoryEvent(event) {
			return false
		}
	}
	for key, search := range state.Searches {
		if !validSubtitleSearchKey(key) || !validSubtitleSearchRecord(search) {
			return false
		}
	}
	for key, record := range state.Records {
		if !validSubtitleRecordKey(key) || !validSubtitleRecord(record) {
			return false
		}
	}
	return state.SubDLDay == "" && state.SubDLDownloads == 0 || validSubtitleDay(state.SubDLDay)
}

func validSubtitleLedgerBounds(state subtitleLedgerState) bool {
	return state.Version >= 1 && state.Version <= subtitleLedgerVersion && len(state.Records) <= subtitleLedgerLimit && len(state.Searches) <= subtitleLedgerLimit && len(state.History) <= subtitleHistoryLimit && (state.Version >= 3 || len(state.History) == 0) && state.SubDLDownloads >= 0 && state.SubDLDownloads <= subDLAutomaticDownloadLimit
}

func validSubtitleSearchKey(key string) bool {
	parts := strings.Split(key, ":")
	return len(parts) == 3 && validSubtitleItemID(parts[0]) && validLanguage(parts[1]) && oneOf(parts[2], "standard", "sdh", "forced")
}

func validSubtitleRecordKey(key string) bool {
	parts := strings.Split(key, ":")
	return len(parts) == 2 && validSubtitleItemID(parts[0]) && validLanguage(parts[1])
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
	return oneOf(record.Outcome, "installed", "no-result", "provider-error") && record.Attempts >= 0 && record.Attempts <= 1000 && record.CheckedAt >= 0 && record.CheckedAt <= time.Now().Add(24*time.Hour).Unix() && record.NextAt >= record.CheckedAt && record.NextAt <= time.Now().Add(8*24*time.Hour).Unix() && len(record.SafeError) <= 256 && record.MediaSize >= 0 && record.MediaModified >= 0
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

func (ledger *subtitleLedger) store(key string, record subtitleRecord) error {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	available := ledger.err == nil && validSubtitleRecord(record) && (len(ledger.state.Records) < subtitleLedgerLimit || ledger.state.Records[key].Fingerprint != "")
	return storeLedgerRecord(ledger.state.Records, key, record, available, func() error {
		return ledger.persist(subtitleLedgerRecordsDocument, key, record)
	})
}

func (ledger *subtitleLedger) search(key string) (subtitleSearchRecord, bool, error) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	record, found := ledger.state.Searches[key]
	return record, found, ledger.err
}

func (ledger *subtitleLedger) storeSearch(key string, record subtitleSearchRecord) error {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	available := ledger.err == nil && validSubtitleSearchRecord(record) && (len(ledger.state.Searches) < subtitleLedgerLimit || ledger.state.Searches[key].Outcome != "")
	return storeLedgerRecord(ledger.state.Searches, key, record, available, func() error {
		return ledger.persist(subtitleLedgerSearchesDocument, key, record)
	})
}

func storeLedgerRecord[T any](records map[string]T, key string, record T, available bool, save func() error) error {
	if !available {
		return errSubtitleLedgerUnavailable
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
	if err := ledger.saveSummary(); err != nil {
		ledger.state.SubDLDay, ledger.state.SubDLDownloads = previousDay, previousCount
		return false
	}
	return true
}
