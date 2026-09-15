package identitycore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

type sessionLoader struct {
	data  []byte
	found bool
	err   error
	name  string
}

func (loader *sessionLoader) Load(name string) ([]byte, bool, error) {
	loader.name = name
	return loader.data, loader.found, loader.err
}

type sessionFixture struct {
	values   map[string]Session
	profiles []SessionProfile
	now      time.Time
	persist  error
	writes   int
	written  map[string]Session
}

func newSessionFixture() *sessionFixture {
	return &sessionFixture{
		values: map[string]Session{}, now: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC),
		profiles: []SessionProfile{{ID: "viewer", Name: "Sam", Remote: true, Secured: true, Revision: 7}},
	}
}

func (fixture *sessionFixture) sessions(options ...func(*SessionConfig)) *Sessions {
	config := SessionConfig{
		Mutex: &sync.RWMutex{}, Values: &fixture.values, File: "sessions.json",
		Persist: func(path string, value any) error {
			fixture.writes++
			if path != "sessions.json" {
				panic("unexpected persistence path")
			}
			fixture.written = CloneSessions(value.(map[string]Session))
			return fixture.persist
		},
		Profiles: func() []SessionProfile { return fixture.profiles },
		Timeouts: func() (time.Duration, time.Duration) { return time.Hour, 12 * time.Hour },
		Now:      func() time.Time { return fixture.now }, NewToken: func() string { return "token" },
	}
	for _, option := range options {
		option(&config)
	}
	return NewSessions(config)
}

func TestLoadSessionsSupportsDocumentsFilesAndLegacyState(t *testing.T) { //nolint:cyclop // One test covers all load adapters and formats.
	t.Parallel()
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	loader := &sessionLoader{data: []byte(`{"raw":{"profileId":"viewer"}}`), found: true}
	loaded, err := LoadSessions("/data/sessions.json", loader, func() time.Time { return now })
	if err != nil || loader.name != "sessions.json" || loaded[SessionKey("raw")].Name != "Legacy device" || loaded[SessionKey("raw")].CreatedAt != now.Unix() {
		t.Fatalf("document load = %#v name=%q err=%v", loaded, loader.name, err)
	}
	missing, err := LoadSessions(filepath.Join(t.TempDir(), "missing.json"), nil, nil)
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing file = %#v, %v", missing, err)
	}
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(path, []byte(`{"legacy-token":"profile"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := LoadSessions(path, nil, func() time.Time { return now })
	entry := legacy[SessionKey("legacy-token")]
	if err != nil || entry.ProfileID != "profile" || entry.Name != "Legacy device" || entry.CreatedAt != now.Unix() || entry.ExpiresAt != now.Add(30*24*time.Hour).Unix() {
		t.Fatalf("legacy load = %#v, %v", legacy, err)
	}
}

func TestLoadSessionsReturnsStorageAndDecodeFailures(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("load failed")
	for name, loader := range map[string]*sessionLoader{
		"not found": {found: false}, "failure": {err: wantErr}, "invalid": {data: []byte(`{"broken":`), found: true},
	} {
		loaded, err := LoadSessions("sessions.json", loader, time.Now)
		if name == "not found" {
			if err != nil || len(loaded) != 0 {
				t.Fatalf("%s = %#v, %v", name, loaded, err)
			}
		} else if err == nil || len(loaded) != 0 {
			t.Fatalf("%s = %#v, %v", name, loaded, err)
		}
	}
	if _, err := LoadSessions(t.TempDir(), nil, time.Now); err == nil {
		t.Fatal("directory read succeeded")
	}
}

func TestSessionCreationPreservesPlayerPolicies(t *testing.T) { //nolint:cyclop // The table covers each independent profile policy.
	t.Parallel()
	for name, profile := range map[string]SessionProfile{
		"missing": {}, "disabled": {ID: "viewer", Disabled: true}, "deleted": {ID: "viewer", Deleted: true},
	} {
		fixture := newSessionFixture()
		fixture.profiles = []SessionProfile{profile}
		if _, err := fixture.sessions().Create("viewer", "device", false, false, ""); !errors.Is(err, ErrProfileNotFound) || fixture.writes != 0 {
			t.Fatalf("%s error = %v writes=%d", name, err, fixture.writes)
		}
	}
	for name, profile := range map[string]SessionProfile{
		"owner": {ID: "viewer", Owner: true, Remote: true, Secured: true},
		"local": {ID: "viewer", Secured: true}, "unsecured": {ID: "viewer", Remote: true},
	} {
		fixture := newSessionFixture()
		fixture.profiles = []SessionProfile{profile}
		if _, err := fixture.sessions().Create("viewer", "device", false, false, "public"); !errors.Is(err, ErrPublicProfile) || fixture.writes != 0 {
			t.Fatalf("%s error = %v writes=%d", name, err, fixture.writes)
		}
	}
	fixture := newSessionFixture()
	for index := 0; index < publicSessionLimit; index++ {
		fixture.values[string(rune('a'+index))] = Session{ProfileID: "viewer", Channel: "public", ExpiresAt: fixture.now.Add(time.Hour).Unix()}
	}
	fixture.values["expired"] = Session{ProfileID: "viewer", Channel: "public", ExpiresAt: fixture.now.Unix()}
	fixture.values["other"] = Session{ProfileID: "other", Channel: "public", ExpiresAt: fixture.now.Add(time.Hour).Unix()}
	fixture.values["private"] = Session{ProfileID: "viewer", ExpiresAt: fixture.now.Add(time.Hour).Unix()}
	if _, err := fixture.sessions().Create("viewer", "device", false, false, "public"); !errors.Is(err, ErrPublicLimit) || fixture.writes != 0 {
		t.Fatalf("public limit = %v writes=%d", err, fixture.writes)
	}
}

func TestSessionCreationCommitsEachSessionKind(t *testing.T) { //nolint:cyclop // Exact state assertions cover each session kind.
	t.Parallel()
	for name, input := range map[string]struct {
		browser, strong bool
		channel         string
		expires         time.Duration
	}{
		"device": {expires: 30 * 24 * time.Hour}, "browser": {browser: true, expires: 12 * time.Hour},
		"public": {browser: true, strong: true, channel: "public", expires: 8 * time.Hour},
	} {
		fixture := newSessionFixture()
		token, err := fixture.sessions().Create("viewer", "  Mozilla Chrome/1 Safari/1  ", input.browser, input.strong, input.channel)
		state := fixture.values[SessionKey("token")]
		wantStrong := int64(0)
		if input.strong {
			wantStrong = fixture.now.Unix()
		}
		if err != nil || token != "token" || fixture.writes != 1 || state.ProfileID != "viewer" || state.Name != "Chrome" || state.CreatedAt != fixture.now.Unix() || state.LastSeen != fixture.now.Unix() || state.ExpiresAt != fixture.now.Add(input.expires).Unix() || state.Browser != input.browser || state.StrongAt != wantStrong || state.Channel != input.channel || state.ProfileRevision != 7 {
			t.Fatalf("%s state = %#v token=%q writes=%d err=%v", name, state, token, fixture.writes, err)
		}
	}
	fixture := newSessionFixture()
	fixture.persist = errors.New("persist failed")
	if token, err := fixture.sessions().Create("viewer", "device", false, false, ""); err == nil || token != "" || len(fixture.values) != 0 || fixture.writes != 1 {
		t.Fatalf("failed create = %q %#v writes=%d err=%v", token, fixture.values, fixture.writes, err)
	}
}

func TestSessionAuthenticationAndRevocation(t *testing.T) { //nolint:cyclop // One lifecycle verifies mutation ordering and persistence rollback.
	t.Parallel()
	fixture := newSessionFixture()
	key := SessionKey("token")
	fixture.values = map[string]Session{key: {ProfileID: "viewer", ExpiresAt: fixture.now.Add(time.Hour).Unix()}, SessionKey("weak"): {ProfileID: "other"}}
	sessions := fixture.sessions()
	if err := sessions.MarkStrong("missing"); !errors.Is(err, ErrCurrentSession) {
		t.Fatalf("missing strong = %v", err)
	}
	if err := sessions.MarkStrong("token"); err != nil || fixture.values[key].StrongAt != fixture.now.Unix() {
		t.Fatalf("strong = %#v, %v", fixture.values[key], err)
	}
	if !sessions.RecentlyAuthenticated("token", time.Minute) || sessions.RecentlyAuthenticated("token", 0) || sessions.RecentlyAuthenticated("missing", time.Minute) || sessions.RecentlyAuthenticated("weak", time.Minute) {
		t.Fatal("recent authentication window is incorrect")
	}
	fixture.values[key] = Session{Channel: "public"}
	if !sessions.Public("token") || sessions.Public("missing") {
		t.Fatal("public session lookup is incorrect")
	}
	if err := sessions.RevokeOthers("missing"); !errors.Is(err, ErrCurrentSession) {
		t.Fatalf("missing revoke others = %v", err)
	}
	if err := sessions.RevokeOthers("token"); err != nil || len(fixture.values) != 1 || fixture.values[key].Channel != "public" {
		t.Fatalf("revoke others = %#v, %v", fixture.values, err)
	}
	if err := sessions.SignOut(""); err != nil || len(fixture.values) != 1 {
		t.Fatalf("empty sign out = %#v, %v", fixture.values, err)
	}
	if err := sessions.SignOut("token"); err != nil || len(fixture.values) != 0 {
		t.Fatalf("sign out = %#v, %v", fixture.values, err)
	}
}

func TestSessionAdminViewsAndFailures(t *testing.T) { //nolint:cyclop // One lifecycle covers admin views and rollback.
	t.Parallel()
	fixture := newSessionFixture()
	active := Session{ProfileID: "viewer", Name: "Chrome", CreatedAt: fixture.now.Add(-time.Hour).Unix(), LastSeen: fixture.now.Unix(), ExpiresAt: fixture.now.Add(time.Hour).Unix()}
	fixture.values = map[string]Session{
		"active": active, "unknown": {ProfileID: "missing", Name: "TV", CreatedAt: fixture.now.Unix(), LastSeen: fixture.now.Unix(), ExpiresAt: fixture.now.Add(time.Hour).Unix()},
		"expired": {ExpiresAt: fixture.now.Unix()}, "inactive": {Browser: true, LastSeen: fixture.now.Add(-2 * time.Hour).Unix(), CreatedAt: fixture.now.Unix(), ExpiresAt: fixture.now.Add(time.Hour).Unix()},
	}
	sessions := fixture.sessions()
	if sessions.Active() != 2 {
		t.Fatalf("active = %d", sessions.Active())
	}
	devices := sessions.Devices()
	if len(devices) != 2 || devices[0].ID != "unknown" || devices[0].Profile != "Viewer" || devices[1].ID != "active" || devices[1].Profile != "Sam" {
		t.Fatalf("devices = %#v", devices)
	}
	if err := sessions.RevokeDevice("missing"); !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("missing device = %v", err)
	}
	fixture.persist = errors.New("persist failed")
	before := CloneSessions(fixture.values)
	if err := sessions.RevokeDevice("active"); err == nil || !reflect.DeepEqual(fixture.values, before) {
		t.Fatalf("failed revoke changed state = %#v, %v", fixture.values, err)
	}
	fixture.persist = nil
	if err := sessions.RevokeDevice("active"); err != nil || len(fixture.values) != 3 {
		t.Fatalf("device revoke = %#v, %v", fixture.values, err)
	}
	if err := sessions.RevokeAll(); err != nil || len(fixture.values) != 0 {
		t.Fatalf("revoke all = %#v, %v", fixture.values, err)
	}
}

func TestSessionHelpersAndInvalidManagers(t *testing.T) {
	t.Parallel()
	if DefaultSessionInactive != 15*time.Minute || DefaultSessionAbsolute != 8*time.Hour {
		t.Fatal("default session timeouts changed")
	}
	now := time.Now().Unix()
	active := Session{ExpiresAt: now + 1}
	for name, session := range map[string]Session{
		"expiry": {ExpiresAt: now}, "inactive": {ExpiresAt: now + 100, Browser: true, LastSeen: now - 60, CreatedAt: now},
		"absolute": {ExpiresAt: now + 100, Browser: true, LastSeen: now, CreatedAt: now - 120},
	} {
		if !SessionExpired(session, now, time.Minute, 2*time.Minute) {
			t.Fatalf("%s did not expire", name)
		}
	}
	if SessionExpired(active, now, time.Minute, 2*time.Minute) {
		t.Fatal("active session expired")
	}
	if SessionExpired(Session{ExpiresAt: now + 100, Browser: true, LastSeen: now, CreatedAt: now}, now, time.Minute, 2*time.Minute) {
		t.Fatal("active browser session expired")
	}
}

func TestActivePublicAndSessionCopies(t *testing.T) {
	t.Parallel()
	now := time.Now().Unix()
	public := map[string]Session{
		"eligible": {ProfileID: "viewer", Channel: "public", ExpiresAt: now + 1},
		"expired":  {ProfileID: "viewer", Channel: "public", ExpiresAt: now},
		"private":  {ProfileID: "viewer", ExpiresAt: now + 1},
		"other":    {ProfileID: "other", Channel: "public", ExpiresAt: now + 1},
	}
	if activePublic(public, "viewer", now) != 1 {
		t.Fatal("active public session count is incorrect")
	}
	original := map[string]Session{"a": {ProfileID: "one"}, "b": {ProfileID: "two", Channel: "public"}, "c": {ProfileID: "two"}}
	clone := CloneSessions(original)
	delete(clone, "a")
	withoutAll := WithoutProfile(original, "two", false)
	withoutPublic := WithoutProfile(original, "two", true)
	if len(original) != 3 || !reflect.DeepEqual(withoutAll, map[string]Session{"a": {ProfileID: "one"}}) || !reflect.DeepEqual(withoutPublic, map[string]Session{"a": {ProfileID: "one"}, "c": {ProfileID: "two"}}) {
		t.Fatalf("session copies changed: original=%#v clone=%#v", original, clone)
	}
}

func TestInvalidSessionManagers(t *testing.T) {
	t.Parallel()
	assertInvalidSessionsFailClosed(t)
	configured := NewSessions(SessionConfig{})
	if !errors.Is(configured.change(nil), ErrCurrentSession) {
		t.Fatal("nil change did not fail closed")
	}
	defaults := newSessionFixture()
	defaultSessions := defaults.sessions(func(config *SessionConfig) {
		config.Now, config.NewToken, config.Timeouts = nil, nil, nil
	})
	if token, err := defaultSessions.Create("viewer", "device", true, false, ""); err != nil || token == "" || defaultSessions.Active() != 1 {
		t.Fatalf("default dependencies = %q, %v", token, err)
	}
}

func assertInvalidSessionsFailClosed(t *testing.T) {
	t.Helper()
	var invalid *Sessions
	if _, err := invalid.Create("", "", false, false, ""); !errors.Is(err, ErrProfileNotFound) || !errors.Is(invalid.MarkStrong(""), ErrCurrentSession) || invalid.RecentlyAuthenticated("", time.Hour) || invalid.Public("") || invalid.Active() != 0 || len(invalid.Devices()) != 0 || !errors.Is(invalid.RevokeAll(), ErrCurrentSession) {
		t.Fatal("invalid manager did not fail closed")
	}
}
