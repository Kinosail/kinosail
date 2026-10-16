package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
)

const (
	subtitleLedgerRecordsDocument  = "subtitle_records.json"
	subtitleLedgerSearchesDocument = "subtitle_searches.json"
	subtitleLedgerDocument         = "subtitle_ledger.json"
	subtitleLedgerLegacyFile       = "subtitle_acquisitions.json"
)

var (
	errSubtitleLedgerInvalid     = errors.New("subtitle acquisition state is invalid")
	errSubtitleLedgerUnavailable = errors.New("subtitle acquisition state is unavailable")
)

type subtitleLedgerSummary struct {
	Version        int                    `json:"version"`
	History        []subtitleHistoryEvent `json:"history,omitempty"`
	SubDLDay       string                 `json:"subdl_day,omitempty"`
	SubDLDownloads int                    `json:"subdl_downloads,omitempty"`
}

func emptySubtitleLedgerState() subtitleLedgerState {
	return subtitleLedgerState{Version: subtitleLedgerVersion, Records: make(map[string]subtitleRecord), Searches: make(map[string]subtitleSearchRecord)}
}

func (ledger *subtitleLedger) open() error {
	for _, name := range []string{subtitleLedgerRecordsDocument, subtitleLedgerSearchesDocument} {
		if err := ledger.db.EnableEntries(name); err != nil {
			return errSubtitleLedgerUnavailable
		}
	}
	state, legacy, err := readLegacySubtitleLedger(ledger.path)
	if err != nil {
		return err
	}
	if legacy {
		return ledger.migrate(state, true)
	}
	var summary subtitleLedgerSummary
	found, err := loadSubtitleLedgerDocument(ledger.db, subtitleLedgerDocument, &summary)
	if err != nil {
		return err
	}
	if !found {
		return ledger.migrate(state, false)
	}
	if _, err := loadSubtitleLedgerDocument(ledger.db, subtitleLedgerRecordsDocument, &state.Records); err != nil {
		return err
	}
	if _, err := loadSubtitleLedgerDocument(ledger.db, subtitleLedgerSearchesDocument, &state.Searches); err != nil {
		return err
	}
	state.Version, state.History, state.SubDLDay, state.SubDLDownloads = summary.Version, summary.History, summary.SubDLDay, summary.SubDLDownloads
	if !validSubtitleLedgerState(state) {
		return errSubtitleLedgerInvalid
	}
	ledger.state = normalizeSubtitleLedgerState(state)
	return nil
}

func loadSubtitleLedgerDocument(store *database.Store, name string, target any) (bool, error) {
	data, found, err := store.Load(name)
	if err != nil {
		return false, errSubtitleLedgerInvalid
	}
	if !found {
		return false, nil
	}
	if decodeSubtitleLedger(bytes.NewReader(data), target) != nil {
		return false, errSubtitleLedgerInvalid
	}
	return true, nil
}

func decodeSubtitleLedger(reader io.Reader, target any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, database.DocumentSizeLimit+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errSubtitleLedgerInvalid
	}
	return nil
}

func (ledger *subtitleLedger) migrate(state subtitleLedgerState, legacy bool) error {
	if ledger.db.SaveJSON(subtitleLedgerRecordsDocument, state.Records) != nil || ledger.db.SaveJSON(subtitleLedgerSearchesDocument, state.Searches) != nil || ledger.db.SaveJSON(subtitleLedgerDocument, summarizeSubtitleLedger(state)) != nil {
		return errSubtitleLedgerUnavailable
	}
	ledger.state = state
	if legacy {
		slog.Info("subtitle acquisition state migrated", "records", len(state.Records), "searches", len(state.Searches))
		ledger.removeLegacyFile()
	}
	return nil
}

func readLegacySubtitleLedger(path string) (subtitleLedgerState, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return emptySubtitleLedgerState(), false, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > database.DocumentSizeLimit {
		return subtitleLedgerState{}, false, errSubtitleLedgerInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return subtitleLedgerState{}, false, errSubtitleLedgerUnavailable
	}
	defer file.Close()
	var state subtitleLedgerState
	if decodeSubtitleLedger(file, &state) != nil || !validSubtitleLedgerState(state) {
		return subtitleLedgerState{}, false, errSubtitleLedgerInvalid
	}
	return normalizeSubtitleLedgerState(migrateSubtitleHistory(state)), true, nil
}

func normalizeSubtitleLedgerState(state subtitleLedgerState) subtitleLedgerState {
	if state.Records == nil {
		state.Records = make(map[string]subtitleRecord)
	}
	if state.Searches == nil {
		state.Searches = make(map[string]subtitleSearchRecord)
	}
	state.Version = subtitleLedgerVersion
	return state
}

func summarizeSubtitleLedger(state subtitleLedgerState) subtitleLedgerSummary {
	return subtitleLedgerSummary{Version: subtitleLedgerVersion, History: state.History, SubDLDay: state.SubDLDay, SubDLDownloads: state.SubDLDownloads}
}

func (ledger *subtitleLedger) removeLegacyFile() {
	if err := os.Remove(ledger.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("legacy subtitle acquisition state was not removed", "error_class", "filesystem")
	}
}

func (ledger *subtitleLedger) persist(name, key string, value any) error {
	if ledger.db == nil {
		return nil
	}
	var err error
	if key == "" {
		err = ledger.db.SaveJSON(name, value)
	} else {
		err = ledger.db.SaveEntry(name, key, value)
	}
	if err != nil {
		slog.Warn("subtitle acquisition state write failed", "document", name, "error", err)
		return errSubtitleLedgerUnavailable
	}
	return nil
}

func (ledger *subtitleLedger) saveSummary() error {
	return ledger.persist(subtitleLedgerDocument, "", summarizeSubtitleLedger(ledger.state))
}

func (ledger *subtitleLedger) saveHistoryRecord(key string, record subtitleRecord, before []subtitleHistoryEvent) error {
	if err := ledger.saveSummary(); err != nil {
		return err
	}
	if err := ledger.persist(subtitleLedgerRecordsDocument, key, record); err != nil {
		after := ledger.state.History
		ledger.state.History = before
		_ = ledger.saveSummary()
		ledger.state.History = after
		return err
	}
	return nil
}
