package identitycore

import (
	"sort"
	"strings"
	"time"
)

func (sessions *Sessions) Devices() []Device {
	if !sessions.valid() {
		return []Device{}
	}
	now := sessions.config.Now().Unix()
	inactive, absolute := sessions.config.Timeouts()
	sessions.config.Mutex.RLock()
	defer sessions.config.Mutex.RUnlock()
	names := make(map[string]string)
	for _, profile := range sessions.config.Profiles() {
		names[profile.ID] = profile.Name
	}
	devices := make([]Device, 0, len(*sessions.config.Values))
	for id, session := range *sessions.config.Values {
		if !SessionExpired(session, now, inactive, absolute) {
			name := names[session.ProfileID]
			if name == "" {
				name = "Viewer"
			}
			devices = append(devices, Device{id, session.Name, name, time.Unix(session.CreatedAt, 0).Local().Format("Jan 2, 2006 3:04 PM")})
		}
	}
	sort.Slice(devices, func(left, right int) bool { return strings.Compare(devices[left].Created, devices[right].Created) == 1 })
	return devices
}

func (sessions *Sessions) RevokeDevice(id string) error {
	return sessions.change(func(values map[string]Session) error {
		if _, found := values[id]; !found {
			return ErrDeviceNotFound
		}
		delete(values, id)
		return nil
	})
}

func (sessions *Sessions) RevokeAll() error {
	return sessions.change(func(values map[string]Session) error {
		clear(values)
		return nil
	})
}

func (sessions *Sessions) change(change func(map[string]Session) error) error {
	if !sessions.valid() || change == nil {
		return ErrCurrentSession
	}
	sessions.config.Mutex.Lock()
	defer sessions.config.Mutex.Unlock()
	values := CloneSessions(*sessions.config.Values)
	if err := change(values); err != nil {
		return err
	}
	return sessions.commit(values)
}

func (sessions *Sessions) commit(values map[string]Session) error {
	if err := sessions.config.Persist(sessions.config.File, values); err != nil {
		return err
	}
	*sessions.config.Values = values
	return nil
}

func (sessions *Sessions) valid() bool {
	return sessions != nil && sessions.config.Mutex != nil && sessions.config.Values != nil && sessions.config.Persist != nil && sessions.config.Profiles != nil
}

func findSessionProfile(profiles []SessionProfile, id string) (SessionProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return SessionProfile{}, false
}

func activePublic(sessions map[string]Session, profileID string, now int64) int {
	count := 0
	for _, session := range sessions {
		if session.ProfileID == profileID && session.Channel == "public" && session.ExpiresAt > now {
			count++
		}
	}
	return count
}

func SessionExpired(session Session, now int64, inactive, absolute time.Duration) bool {
	if session.ExpiresAt <= now {
		return true
	}
	if !session.Browser {
		return false
	}
	if session.LastSeen+int64(inactive/time.Second) <= now {
		return true
	}
	return session.CreatedAt+int64(absolute/time.Second) <= now
}

func CloneSessions(sessions map[string]Session) map[string]Session {
	copy := make(map[string]Session, len(sessions))
	for token, session := range sessions {
		copy[token] = session
	}
	return copy
}

func WithoutProfile(sessions map[string]Session, profileID string, publicOnly bool) map[string]Session {
	copy := CloneSessions(sessions)
	for token, session := range copy {
		if session.ProfileID == profileID && (!publicOnly || session.Channel == "public") {
			delete(copy, token)
		}
	}
	return copy
}

// WithoutPublicSessions returns a copy without public Internet sessions.
func WithoutPublicSessions(sessions map[string]Session) map[string]Session {
	copy := CloneSessions(sessions)
	for token, session := range copy {
		if session.Channel == "public" {
			delete(copy, token)
		}
	}
	return copy
}
