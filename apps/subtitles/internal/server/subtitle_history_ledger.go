package server

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const subtitleHistoryLimit = 200

type subtitleHistoryEvidence struct {
	Reason               string   `json:"reason,omitempty"`
	Score                *int     `json:"score,omitempty"`
	ReleaseMatch         *float64 `json:"releaseMatch,omitempty"`
	PreviousSource       string   `json:"previousSource,omitempty"`
	PreviousScore        *int     `json:"previousScore,omitempty"`
	PreviousReleaseMatch *float64 `json:"previousReleaseMatch,omitempty"`
}

type subtitleHistoryEvent struct {
	subtitleHistoryEvidence
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
	return len(parts) == 2 && validSubtitleItemID(parts[0]) && validLanguage(parts[1]) && oneOf(event.Action, "added", "updated", "restored", "recorded") && subtitleHistorySource(event.Source) && event.InstalledAt > 0 && event.InstalledAt <= time.Now().Add(24*time.Hour).Unix() && validSubtitleHistoryEvidence(event)
}

func migrateSubtitleHistory(state subtitleLedgerState) subtitleLedgerState {
	if state.Version >= 3 {
		return state
	}
	for key, record := range state.Records {
		if oneOf(record.Source, "subdl", "opensubtitles", "subsource") && record.InstalledAt > 0 {
			state.History = append(state.History, subtitleHistoryEvent{Key: key, Action: "recorded", Source: record.Source, InstalledAt: record.InstalledAt, subtitleHistoryEvidence: subtitleHistoryEvidence{Reason: "legacy", Score: &record.Score, ReleaseMatch: &record.ReleaseMatch}})
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

func (ledger *subtitleLedger) storeHistory(key string, record subtitleRecord, action, reason string) error { //nolint:cyclop // Snapshot validation and durable rollback stay in one atomic ledger operation.
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	previous, found := ledger.state.Records[key]
	event := subtitleHistoryEvent{Key: key, Action: action, Source: record.Source, InstalledAt: record.InstalledAt, subtitleHistoryEvidence: subtitleHistoryEvidence{Reason: reason}}
	if oneOf(reason, "missing", "embedded", "higher-score", "exact-hash") {
		event.Score, event.ReleaseMatch = &record.Score, &record.ReleaseMatch
	}
	if reason == "higher-score" && found {
		event.PreviousSource, event.PreviousScore, event.PreviousReleaseMatch = previous.Source, &previous.Score, &previous.ReleaseMatch
	}
	if ledger.err != nil || !validSubtitleRecord(record) || !validSubtitleHistoryEvent(event) || len(ledger.state.Records) >= subtitleLedgerLimit && ledger.state.Records[key].Fingerprint == "" {
		return errors.New("subtitle acquisition state is unavailable")
	}
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
	if ledger.publish != nil {
		ledger.publish()
	}
	return nil
}

func (ledger *subtitleLedger) history() ([]subtitleHistoryEvent, error) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	return append([]subtitleHistoryEvent(nil), ledger.state.History...), ledger.err
}

func validSubtitleHistoryEvidence(event subtitleHistoryEvent) bool { //nolint:cyclop // Persisted explanations must agree with their action and score snapshots.
	if !validSubtitleHistoryMatch(event.Score, event.ReleaseMatch) || !validSubtitleHistoryMatch(event.PreviousScore, event.PreviousReleaseMatch) {
		return false
	}
	previous := event.PreviousSource != "" || event.PreviousScore != nil
	if event.Reason == "higher-score" {
		return event.Action == "updated" && oneOf(event.Source, "subdl", "opensubtitles", "subsource") && subtitleHistorySource(event.PreviousSource) && event.Score != nil && event.PreviousScore != nil && *event.Score >= *event.PreviousScore+subtitleUpgradeGain
	}
	if previous {
		return false
	}
	switch event.Reason {
	case "":
		return event.Score == nil
	case "legacy":
		return event.Action == "recorded"
	case "missing":
		return event.Action == "added" && oneOf(event.Source, "subdl", "opensubtitles", "subsource") && event.Score != nil
	case "embedded":
		return event.Action == "added" && event.Source == "embedded" && event.Score != nil && *event.Score == 100 && *event.ReleaseMatch == 1
	case "exact-hash":
		return event.Action == "updated" && oneOf(event.Source, "subdl", "opensubtitles", "subsource") && event.Score != nil && *event.Score == 100 && *event.ReleaseMatch == 1
	case "manual":
		return oneOf(event.Action, "added", "updated") && event.Score == nil
	case "restore":
		return event.Action == "restored" && event.Score == nil
	default:
		return false
	}
}

func validSubtitleHistoryMatch(score *int, release *float64) bool {
	return score == nil && release == nil || score != nil && release != nil && *score >= 0 && *score <= 100 && *release >= 0 && *release <= 1
}
