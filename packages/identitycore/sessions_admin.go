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
	sessions.config.Mutex.RLock()
	defer sessions.config.Mutex.RUnlock()
	names := make(map[string]string)
	for _, profile := range sessions.config.Profiles() {
		names[profile.ID] = profile.Name
	}
	devices := make([]Device, 0, len(*sessions.config.Values))
	for id, session := range *sessions.config.Values {
		inactive, absolute := sessions.TimeoutsFor(session)
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
	return sessions.remove(func(values map[string]Session) ([]string, error) {
		if _, found := values[id]; !found {
			return nil, ErrDeviceNotFound
		}
		return []string{id}, nil
	})
}

func (sessions *Sessions) RevokeAll() error {
	return sessions.replace(func(map[string]Session) (map[string]Session, error) { return map[string]Session{}, nil })
}

func (sessions *Sessions) remove(selectKeys func(map[string]Session) ([]string, error)) error {
	if !sessions.valid() {
		return ErrCurrentSession
	}
	sessions.config.Mutex.Lock()
	keys, err := selectKeys(sessions.values())
	if err != nil {
		sessions.config.Mutex.Unlock()
		return err
	}
	change := applySessionChange(sessions.values(), nil, keys)
	sessions.config.Mutex.Unlock()
	if change.empty() {
		return nil
	}
	return sessions.persistChange(change)
}

func (sessions *Sessions) replace(next func(map[string]Session) (map[string]Session, error)) error {
	if !sessions.valid() {
		return ErrCurrentSession
	}
	sessions.config.Mutex.Lock()
	defer sessions.config.Mutex.Unlock()
	values, err := next(sessions.values())
	if err != nil {
		return err
	}
	if err := sessions.config.Persist(sessions.config.File, values); err != nil {
		return err
	}
	*sessions.config.Values = values
	return nil
}

func (sessions *Sessions) values() map[string]Session {
	if *sessions.config.Values == nil {
		*sessions.config.Values = make(map[string]Session)
	}
	return *sessions.config.Values
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

func activePublic(sessions map[string]Session, profileID string, now int64, inactive, absolute time.Duration) int {
	count := 0
	for _, session := range sessions {
		if session.ProfileID == profileID && session.Channel == "public" && !SessionExpired(session, now, inactive, absolute) {
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
	seconds := int64(inactive / time.Second)
	if session.InactiveSeconds > 0 {
		seconds = min(seconds, session.InactiveSeconds)
	}
	if session.LastSeen+seconds <= now {
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

func WithoutPublicSessions(sessions map[string]Session) map[string]Session {
	copy := CloneSessions(sessions)
	for token, session := range copy {
		if session.Channel == "public" {
			delete(copy, token)
		}
	}
	return copy
}
