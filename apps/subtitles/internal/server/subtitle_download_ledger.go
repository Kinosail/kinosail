package server

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const subtitleDownloadHistoryLimit = 200

type subtitleDownloadEvent struct {
	Key         string `json:"key"`
	Source      string `json:"source"`
	InstalledAt int64  `json:"installedAt"`
}

func subtitleDownloadSource(source string) bool {
	return oneOf(source, "subdl", "opensubtitles", "subsource")
}

func validSubtitleDownloadEvent(event subtitleDownloadEvent) bool {
	parts := strings.Split(event.Key, ":")
	return len(parts) == 2 && validSubtitleItemID(parts[0]) && validLanguage(parts[1]) && subtitleDownloadSource(event.Source) && event.InstalledAt > 0 && event.InstalledAt <= time.Now().Add(24*time.Hour).Unix()
}

func migrateSubtitleDownloads(state subtitleLedgerState) subtitleLedgerState {
	if state.Version >= subtitleLedgerVersion {
		return state
	}
	for key, record := range state.Records {
		if subtitleDownloadSource(record.Source) && record.InstalledAt > 0 {
			state.Downloads = append(state.Downloads, subtitleDownloadEvent{Key: key, Source: record.Source, InstalledAt: record.InstalledAt})
		}
	}
	sort.Slice(state.Downloads, func(i, j int) bool {
		if state.Downloads[i].InstalledAt == state.Downloads[j].InstalledAt {
			return state.Downloads[i].Key < state.Downloads[j].Key
		}
		return state.Downloads[i].InstalledAt < state.Downloads[j].InstalledAt
	})
	if len(state.Downloads) > subtitleDownloadHistoryLimit {
		state.Downloads = state.Downloads[len(state.Downloads)-subtitleDownloadHistoryLimit:]
	}
	return state
}

func (ledger *subtitleLedger) storeDownload(key string, record subtitleRecord) error {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	event := subtitleDownloadEvent{Key: key, Source: record.Source, InstalledAt: record.InstalledAt}
	if ledger.err != nil || !validSubtitleRecord(record) || !validSubtitleDownloadEvent(event) || len(ledger.state.Records) >= subtitleLedgerLimit && ledger.state.Records[key].Fingerprint == "" {
		return errors.New("subtitle acquisition state is unavailable")
	}
	previous, found := ledger.state.Records[key]
	before := ledger.state.Downloads
	ledger.state.Records[key] = record
	ledger.state.Downloads = append(append([]subtitleDownloadEvent(nil), before...), event)
	if len(ledger.state.Downloads) > subtitleDownloadHistoryLimit {
		ledger.state.Downloads = ledger.state.Downloads[1:]
	}
	if err := ledger.save(); err != nil {
		if found {
			ledger.state.Records[key] = previous
		} else {
			delete(ledger.state.Records, key)
		}
		ledger.state.Downloads = before
		return err
	}
	return nil
}

func (ledger *subtitleLedger) downloads() ([]subtitleDownloadEvent, error) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	return append([]subtitleDownloadEvent(nil), ledger.state.Downloads...), ledger.err
}
