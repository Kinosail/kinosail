package server

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const subtitleHistoryLimit = 200

type subtitleHistoryEvent struct {
	Key         string `json:"key"`
	Action      string `json:"action"`
	Source      string `json:"source"`
	InstalledAt int64  `json:"installedAt"`
}

func subtitleHistorySource(source string) bool {
	return oneOf(source, "subdl", "opensubtitles", "subsource", "embedded", "external", "ocr", "transcription")
}

func validSubtitleHistoryEvent(event subtitleHistoryEvent) bool {
	parts := strings.Split(event.Key, ":")
	return len(parts) == 2 && validSubtitleItemID(parts[0]) && validLanguage(parts[1]) && oneOf(event.Action, "added", "updated", "restored") && subtitleHistorySource(event.Source) && event.InstalledAt > 0 && event.InstalledAt <= time.Now().Add(24*time.Hour).Unix()
}

func migrateSubtitleHistory(state subtitleLedgerState) subtitleLedgerState {
	if state.Version >= subtitleLedgerVersion {
		return state
	}
	for key, record := range state.Records {
		if oneOf(record.Source, "subdl", "opensubtitles", "subsource") && record.InstalledAt > 0 {
			state.History = append(state.History, subtitleHistoryEvent{Key: key, Action: "added", Source: record.Source, InstalledAt: record.InstalledAt})
		}
	}
	sort.Slice(state.History, func(i, j int) bool {
		if state.History[i].InstalledAt == state.History[j].InstalledAt {
			return state.History[i].Key < state.History[j].Key
		}
		return state.History[i].InstalledAt < state.History[j].InstalledAt
	})
	if len(state.History) > subtitleHistoryLimit {
		state.History = state.History[len(state.History)-subtitleHistoryLimit:]
	}
	return state
}

func (ledger *subtitleLedger) storeHistory(key string, record subtitleRecord, action string) error {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	event := subtitleHistoryEvent{Key: key, Action: action, Source: record.Source, InstalledAt: record.InstalledAt}
	if ledger.err != nil || !validSubtitleRecord(record) || !validSubtitleHistoryEvent(event) || len(ledger.state.Records) >= subtitleLedgerLimit && ledger.state.Records[key].Fingerprint == "" {
		return errors.New("subtitle acquisition state is unavailable")
	}
	previous, found := ledger.state.Records[key]
	before := ledger.state.History
	ledger.state.Records[key] = record
	ledger.state.History = append(append([]subtitleHistoryEvent(nil), before...), event)
	if len(ledger.state.History) > subtitleHistoryLimit {
		ledger.state.History = ledger.state.History[1:]
	}
	if err := ledger.save(); err != nil {
		if found {
			ledger.state.Records[key] = previous
		} else {
			delete(ledger.state.Records, key)
		}
		ledger.state.History = before
		return err
	}
	return nil
}

func (ledger *subtitleLedger) history() ([]subtitleHistoryEvent, error) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	return append([]subtitleHistoryEvent(nil), ledger.state.History...), ledger.err
}
