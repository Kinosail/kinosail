package catalog

import "time"

// ProgressRevision applies one ordered playback event.
func ProgressRevision(seconds float64, watched *bool, session string, revision uint64, now time.Time) ProgressChange {
	return func(state PlaybackState) (PlaybackState, bool, error) {
		if session != "" && revision > 0 && state.Session == session && revision <= state.Revision {
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
