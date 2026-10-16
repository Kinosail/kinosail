package identitycore

import (
	"sync"
	"time"
)

const (
	SessionUseInterval = 60
	APIKeyUseInterval  = 3600
)

type AccessUse struct {
	mu       sync.RWMutex
	sessions map[string]int64
	apiKeys  map[string]int64
}

func (use *AccessUse) Session(key string, session Session, now int64) Session {
	if !session.Browser || session.LastSeen+SessionUseInterval > now {
		return session
	}
	use.mu.RLock()
	seen := use.sessions[key]
	use.mu.RUnlock()
	session.LastSeen = max(session.LastSeen, seen)
	return session
}

func (use *AccessUse) APIKey(key string, api APIKey, now int64) APIKey {
	if !apiKeyUseDue(api, now) {
		return api
	}
	use.mu.RLock()
	used := use.apiKeys[key]
	use.mu.RUnlock()
	api.LastUsed = max(api.LastUsed, used)
	return api
}

func (use *AccessUse) Record(key string, session Session, sessionFound bool, api APIKey, apiFound bool, now int64) {
	sessionDue := sessionFound && session.Browser && session.LastSeen+SessionUseInterval <= now
	apiDue := apiFound && apiKeyUseDue(api, now)
	if !sessionDue && !apiDue {
		return
	}
	use.mu.Lock()
	defer use.mu.Unlock()
	if sessionDue {
		if use.sessions == nil {
			use.sessions = make(map[string]int64)
		}
		use.sessions[key] = now
	}
	if apiDue {
		if use.apiKeys == nil {
			use.apiKeys = make(map[string]int64)
		}
		use.apiKeys[key] = now
	}
}

func (use *AccessUse) Merge(sessions map[string]Session, keys map[string]APIKey) (bool, bool) {
	use.mu.Lock()
	defer use.mu.Unlock()
	sessionsChanged, keysChanged := false, false
	for key, seen := range use.sessions {
		if session, found := sessions[key]; found && seen > session.LastSeen {
			session.LastSeen = seen
			sessions[key], sessionsChanged = session, true
		}
	}
	for key, used := range use.apiKeys {
		if api, found := keys[key]; found && used > api.LastUsed {
			api.LastUsed = used
			keys[key], keysChanged = api, true
		}
	}
	clear(use.sessions)
	clear(use.apiKeys)
	return sessionsChanged, keysChanged
}

func (use *AccessUse) APIKeys(keys map[string]APIKey) map[string]APIKey {
	use.mu.RLock()
	defer use.mu.RUnlock()
	if len(use.apiKeys) == 0 {
		return keys
	}
	current := CloneAPIKeys(keys)
	for key, used := range use.apiKeys {
		if api, found := current[key]; found && used > api.LastUsed {
			api.LastUsed = used
			current[key] = api
		}
	}
	return current
}

func apiKeyUseDue(api APIKey, now int64) bool {
	return api.LastUsed == 0 || api.LastUsed+APIKeyUseInterval <= now
}

func SessionTimeouts(session Session, timeouts, publicTimeouts func() (time.Duration, time.Duration)) (time.Duration, time.Duration) {
	if session.Channel == "public" && publicTimeouts != nil {
		return publicTimeouts()
	}
	if timeouts == nil {
		return DefaultSessionInactive, DefaultSessionAbsolute
	}
	return timeouts()
}

func RecentlyStrong(session Session, maximumAge time.Duration, now time.Time) bool {
	return session.StrongAt != 0 && time.Unix(session.StrongAt, 0).Add(maximumAge).After(now)
}
