package server

import (
	"errors"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

var errSessionTimeoutsInvalid = errors.New("session timeouts must be 0.25–8760 inactive hours and 4–8760 absolute hours, with inactivity no longer than the absolute limit")

func (store *settingsStore) changeSessionTimeouts(inactive, absolute float64, public bool) error {
	if !validSessionTimeouts(inactive, absolute) {
		return errSessionTimeoutsInvalid
	}
	return store.updateSessionTimeouts(inactive, absolute, public)
}

func (store *settingsStore) resetPublicSessionTimeouts() error {
	return store.updateSessionTimeouts(0, 0, true)
}

func (store *settingsStore) updateSessionTimeouts(inactive, absolute float64, public bool) error {
	// Request validation/issuance already locks profiles before reading settings.
	if store.profiles != nil {
		store.profiles.mu.Lock()
		defer store.profiles.mu.Unlock()
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	next := store.value
	if public {
		next.PublicSessionInactiveHours, next.PublicSessionAbsoluteHours = inactive, absolute
	} else {
		next.SessionInactiveHours, next.SessionAbsoluteHours = inactive, absolute
	}
	if store.profiles == nil {
		if err := store.save(next); err != nil {
			return err
		}
	} else if err := store.commitSessionTimeouts(next); err != nil {
		return err
	}
	store.value = next
	return nil
}

func (store *settingsStore) commitSessionTimeouts(next installationSettings) error {
	profiles := store.profiles
	sessions := identitycore.CloneSessions(profiles.sessions)
	now := time.Now().Unix()
	for key, session := range sessions {
		oldInactive, oldAbsolute := timeoutDurations(store.value, session.Channel)
		inactive, absolute := timeoutDurations(next, session.Channel)
		if session.Channel == "public" && !session.Browser {
			absolute = publicSessionLifetime(next)
		}
		limited, valid := identitycore.LimitSession(session, now, oldInactive, oldAbsolute, inactive, absolute)
		if valid {
			sessions[key] = limited
		} else {
			delete(sessions, key)
		}
	}
	if profiles.database != nil {
		if err := profiles.database.SaveJSONBatch(map[string]any{"settings.json": next, "sessions.json": sessions}); err != nil {
			return err
		}
	} else {
		// File-only adapters cannot transact two files. Pin durable ceilings first;
		// a failed settings write leaves the previous policy and stricter sessions.
		if err := profiles.persist(profiles.sessionFile, sessions); err != nil {
			return err
		}
		profiles.sessions = sessions
		if err := store.save(next); err != nil {
			return err
		}
	}
	profiles.sessions = sessions
	return nil
}

func timeoutDurations(settings installationSettings, channel string) (time.Duration, time.Duration) {
	inactive, absolute := sessionTimeoutHours(settings)
	if channel == "public" {
		inactive, absolute = publicSessionTimeoutHours(settings)
	}
	return time.Duration(inactive * float64(time.Hour)), time.Duration(absolute * float64(time.Hour))
}
