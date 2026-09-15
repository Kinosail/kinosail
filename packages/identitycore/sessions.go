package identitycore

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DefaultSessionInactive time.Duration = 900_000_000_000
	DefaultSessionAbsolute time.Duration = 28_800_000_000_000
	publicSessionLimit                   = 10
)

var (
	ErrProfileNotFound = errors.New("viewer profile was not found")
	ErrPublicProfile   = errors.New("viewer profile is not available for public access")
	ErrPublicLimit     = errors.New("too many public sessions")
	ErrCurrentSession  = errors.New("current session was not found")
	ErrDeviceNotFound  = errors.New("device session was not found")
)

type Session struct {
	ManagementDevice string `json:"managementDevice,omitempty"`
	ProfileID        string `json:"profileId"`
	ExpiresAt        int64  `json:"expiresAt"`
	Name             string `json:"name,omitempty"`
	CreatedAt        int64  `json:"createdAt,omitempty"`
	LastSeen         int64  `json:"lastSeen,omitempty"`
	Browser          bool   `json:"browser,omitempty"`
	StrongAt         int64  `json:"strongAt,omitempty"`
	Channel          string `json:"channel,omitempty"`
	ProfileRevision  uint64 `json:"profileRevision,omitempty"`
}

type Device struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Profile string `json:"profile"`
	Created string `json:"created"`
}

type SessionProfile struct {
	ID, Name                         string
	Owner, Remote, Secured, Disabled bool
	Deleted                          bool
	Revision                         uint64
}

type SessionConfig struct {
	Mutex    *sync.RWMutex
	Values   *map[string]Session
	File     string
	Persist  func(string, any) error
	Profiles func() []SessionProfile
	Timeouts func() (time.Duration, time.Duration)
	Now      func() time.Time
	NewToken func() string
}

type Sessions struct{ config SessionConfig }

type SessionLoader interface {
	Load(string) ([]byte, bool, error)
}

func NewSessions(config SessionConfig) *Sessions {
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewToken == nil {
		config.NewToken = rand.Text
	}
	if config.Timeouts == nil {
		config.Timeouts = func() (time.Duration, time.Duration) { return DefaultSessionInactive, DefaultSessionAbsolute }
	}
	return &Sessions{config: config}
}

func LoadSessions(path string, database SessionLoader, now func() time.Time) (map[string]Session, error) {
	if now == nil {
		now = time.Now
	}
	sessions := make(map[string]Session)
	data, found, err := loadSessionData(path, database)
	if !found && err == nil {
		return sessions, nil
	}
	if err != nil {
		return sessions, err
	}
	if err := json.Unmarshal(data, &sessions); err == nil {
		NormalizeSessions(sessions, now)
		return sessions, nil
	}
	legacy := make(map[string]string)
	if err := json.Unmarshal(data, &legacy); err != nil {
		return sessions, err
	}
	created := now()
	for token, profileID := range legacy {
		sessions[SessionKey(token)] = Session{ProfileID: profileID, ExpiresAt: created.Add(30 * 24 * time.Hour).Unix(), Name: "Legacy device", CreatedAt: created.Unix()}
	}
	return sessions, nil
}

func loadSessionData(path string, database SessionLoader) ([]byte, bool, error) {
	if database != nil {
		return database.Load(filepath.Base(path))
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}

func (sessions *Sessions) Create(profileID, name string, browser, strong bool, channel string) (string, error) {
	return sessions.create(profileID, name, browser, strong, channel, nil, "")
}

// CreatePublicGrant checks approval's Profile revision under the same lock as issuance.
func (sessions *Sessions) CreatePublicGrant(profileID, name string, browser bool, revision uint64) (string, error) {
	return sessions.create(profileID, name, browser, true, "public", &revision, "")
}

func (sessions *Sessions) create(profileID, name string, browser, strong bool, channel string, revision *uint64, deviceKey string) (string, error) { //nolint:cyclop // One lock protects profile policy, limits, approval revision, and persistence.
	if !sessions.valid() {
		return "", ErrProfileNotFound
	}
	if channel != "" && channel != "compatibility" && channel != "public" {
		return "", ErrSessionKind
	}
	now := sessions.config.Now()
	sessions.config.Mutex.Lock()
	defer sessions.config.Mutex.Unlock()
	profile, found := findSessionProfile(sessions.config.Profiles(), profileID)
	if !found || profile.Disabled || profile.Deleted {
		return "", ErrProfileNotFound
	}
	if deviceKey != "" && (!validManagementKey(deviceKey) || !profile.Owner || !profile.Secured || channel != "") {
		return "", ErrSessionKind
	}
	if revision != nil && profile.Revision != *revision {
		return "", ErrPublicProfile
	}
	if channel == "public" && (profile.Owner || !profile.Remote || !profile.Secured) {
		return "", ErrPublicProfile
	}
	if channel == "public" && activePublic(*sessions.config.Values, profileID, now.Unix()) >= publicSessionLimit {
		return "", ErrPublicLimit
	}
	expires := now.Add(30 * 24 * time.Hour)
	if browser {
		_, absolute := sessions.config.Timeouts()
		expires = now.Add(absolute)
	}
	if channel == "public" {
		expires = now.Add(8 * time.Hour)
	}
	strongAt := int64(0)
	if strong {
		strongAt = now.Unix()
	}
	token := sessions.config.NewToken()
	if !ValidSessionToken(token) {
		return "", ErrSessionToken
	}
	values := CloneSessions(*sessions.config.Values)
	values[SessionKey(token)] = Session{ManagementDevice: deviceKey, ProfileID: profileID, ExpiresAt: expires.Unix(), Name: CleanDeviceName(name), CreatedAt: now.Unix(), LastSeen: now.Unix(), Browser: browser, StrongAt: strongAt, Channel: channel, ProfileRevision: profile.Revision}
	if err := sessions.config.Persist(sessions.config.File, values); err != nil {
		return "", err
	}
	*sessions.config.Values = values
	return token, nil
}

func (sessions *Sessions) MarkStrong(token string) error {
	if !sessions.valid() {
		return ErrCurrentSession
	}
	key := SessionKey(token)
	sessions.config.Mutex.Lock()
	defer sessions.config.Mutex.Unlock()
	current, found := (*sessions.config.Values)[key]
	if !found {
		return ErrCurrentSession
	}
	current.StrongAt = sessions.config.Now().Unix()
	values := CloneSessions(*sessions.config.Values)
	values[key] = current
	return sessions.commit(values)
}

func (sessions *Sessions) RecentlyAuthenticated(token string, maximumAge time.Duration) bool {
	if !sessions.valid() {
		return false
	}
	sessions.config.Mutex.RLock()
	defer sessions.config.Mutex.RUnlock()
	current, found := (*sessions.config.Values)[SessionKey(token)]
	if !found {
		return false
	}
	if current.StrongAt == 0 {
		return false
	}
	return time.Unix(current.StrongAt, 0).Add(maximumAge).After(sessions.config.Now())
}

func (sessions *Sessions) Public(token string) bool {
	if !sessions.valid() {
		return false
	}
	sessions.config.Mutex.RLock()
	defer sessions.config.Mutex.RUnlock()
	return (*sessions.config.Values)[SessionKey(token)].Channel == "public"
}

func (sessions *Sessions) SignOut(token string) error {
	if token == "" {
		return nil
	}
	return sessions.change(func(values map[string]Session) error {
		delete(values, SessionKey(token))
		return nil
	})
}

func (sessions *Sessions) Active() int {
	if !sessions.valid() {
		return 0
	}
	now := sessions.config.Now().Unix()
	inactive, absolute := sessions.config.Timeouts()
	sessions.config.Mutex.RLock()
	defer sessions.config.Mutex.RUnlock()
	count := 0
	for _, session := range *sessions.config.Values {
		if !SessionExpired(session, now, inactive, absolute) {
			count++
		}
	}
	return count
}

func (sessions *Sessions) RevokeOthers(token string) error {
	key := SessionKey(token)
	return sessions.change(func(values map[string]Session) error {
		current, found := values[key]
		if !found {
			return ErrCurrentSession
		}
		clear(values)
		values[key] = current
		return nil
	})
}
