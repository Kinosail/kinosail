package catalog

import (
	"strings"
	"time"
)

// ProgressWatched records an explicit watched choice and closes its current page.
// An omitted session preserves the legacy watched operation.
func ProgressWatched(watched bool, session string, now time.Time) ProgressChange {
	return func(state PlaybackState) (PlaybackState, bool, error) {
		if session != "" && (len(session) < 8 || len(session) > 64 || strings.IndexFunc(session, invalidWatchedSessionCharacter) >= 0) {
			return state, false, ErrInvalidProgressState
		}
		state.Seconds, state.Watched, state.Revision, state.Updated = 0, watched, 0, now.UTC()
		if session != "" {
			state.Session = session
		}
		return state, true, nil
	}
}

func invalidWatchedSessionCharacter(character rune) bool {
	return character != '-' && character != '_' && (character < '0' || character > '9') && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z')
}

// ProgressRevision applies one ordered playback event.
func ProgressRevision(seconds float64, watched *bool, session string, revision uint64, now time.Time) ProgressChange {
	return func(state PlaybackState) (PlaybackState, bool, error) {
		// An explicit watched choice has no playback-event revision and closes
		// its page. Ordinary completion retains a revision so replay can advance.
		if session != "" && state.Session == session && (state.Watched && state.Revision == 0 || revision > 0 && revision <= state.Revision) {
			return state, false, nil
		}
		state.Seconds, state.Updated = seconds, now.UTC()
		if session != "" && revision > 0 {
			state.Session, state.Revision = session, revision
		}
		if seconds > 0 {
			state.Watched, state.Dismissed = false, false
		}
		if watched != nil {
			state.Watched = *watched
		}
		return state, true, nil
	}
}

// ProgressAudit identifies transitions from a committed playback event.
func ProgressAudit(previous, state PlaybackState, seconds float64, watched *bool, accepted bool, err error) (started, completed bool) {
	if err != nil || !accepted {
		return false, false
	}
	started = seconds > 0 && (previous.Updated.IsZero() || state.Updated.Sub(previous.Updated) >= 30*time.Minute || seconds+30 < previous.Seconds)
	completed = watched != nil && *watched && !previous.Watched
	return started, completed
}
