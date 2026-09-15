package identitycore

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIssueAPIKeyValidatesBeforeCommit(t *testing.T) {
	t.Parallel()
	commits := 0
	commit := func(string, APIKey) error { commits++; return nil }
	for _, input := range []struct {
		profileID, name string
		scopes          []string
		commit          func(string, APIKey) error
	}{
		{"", "Key", []string{"library"}, commit},
		{strings.Repeat("i", maxIdentityLength+1), "Key", []string{"library"}, commit},
		{"owner", "", []string{"library"}, commit},
		{"owner", strings.Repeat("n", 81), []string{"library"}, commit},
		{"owner", "Key", nil, commit},
		{"owner", "Key", []string{"library", "write", "stream", "download", "admin", "home-assistant"}, commit},
		{"owner", "Key", []string{"unknown"}, commit},
		{"owner", "Key", []string{"library"}, nil},
	} {
		if secret, err := IssueAPIKey(input.profileID, input.name, input.scopes, false, input.commit); err == nil || secret != "" {
			t.Fatalf("invalid issuance = %q, %v", secret, err)
		}
	}
	if commits != 0 {
		t.Fatalf("invalid issuance committed %d keys", commits)
	}
}

func TestIssueAPIKeyAcceptsExactInputBounds(t *testing.T) {
	t.Parallel()
	profileID := strings.Repeat("i", maxIdentityLength)
	name := strings.Repeat("n", 80)
	scopes := []string{"library", "write", "stream", "download", "admin"}
	var committed APIKey
	if _, err := IssueAPIKey(profileID, name, scopes, false, func(_ string, key APIKey) error {
		committed = key
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if committed.ProfileID != profileID || committed.Name != name || len(committed.Scopes) != len(scopes) {
		t.Fatalf("bounded key = %#v", committed)
	}
	if lifetime := committed.ExpiresAt - committed.CreatedAt; lifetime != int64((30*24*time.Hour)/time.Second) {
		t.Fatalf("temporary key lifetime = %d", lifetime)
	}
}

func TestLoadedAPIKeys(t *testing.T) {
	t.Parallel()
	keys := map[string]APIKey{"legacy": {CreatedAt: 100}, "persistent": {CreatedAt: 200, Persistent: true}, "current": {CreatedAt: 300, ExpiresAt: 400}}
	loaded, err := LoadedAPIKeys(keys, true, nil)
	if err != nil || loaded["legacy"].ExpiresAt != 100+int64((30*24*time.Hour)/time.Second) || loaded["persistent"].ExpiresAt != 0 || loaded["current"].ExpiresAt != 400 {
		t.Fatalf("loaded keys = %#v, %v", loaded, err)
	}
	missing := map[string]APIKey{}
	if loaded, err := LoadedAPIKeys(missing, false, nil); err != nil || len(loaded) != 0 {
		t.Fatalf("missing keys = %#v, %v", loaded, err)
	}
	wantErr := errors.New("load failed")
	if loaded, err := LoadedAPIKeys(keys, true, wantErr); !errors.Is(err, wantErr) || loaded == nil {
		t.Fatalf("load error = %#v, %v", loaded, err)
	}
}

func TestIssueAPIKeyCommitsTemporaryAndPersistentKeys(t *testing.T) { //nolint:cyclop // Assertions cover each committed field.
	t.Parallel()
	var temporary APIKey
	secret, err := IssueAPIKey("owner", "Automation", []string{"library"}, false, func(value string, key APIKey) error {
		if value == "" {
			t.Fatal("empty secret was committed")
		}
		temporary = key
		return nil
	})
	if err != nil || !strings.HasPrefix(secret, "ks_") || temporary.Persistent || temporary.ExpiresAt <= temporary.CreatedAt {
		t.Fatalf("temporary issuance = %q, %#v, %v", secret, temporary, err)
	}
	var persistent APIKey
	_, err = IssueAPIKey("owner", "Home Assistant", []string{"home-assistant"}, true, func(_ string, key APIKey) error { persistent = key; return nil })
	if err != nil || !persistent.Persistent || persistent.ExpiresAt != 0 {
		t.Fatalf("persistent issuance = %#v, %v", persistent, err)
	}
	wantErr := errors.New("persistence failed")
	if secret, err := IssueAPIKey("owner", "Automation", []string{"admin"}, false, func(string, APIKey) error { return wantErr }); !errors.Is(err, wantErr) || secret != "" {
		t.Fatalf("failed issuance = %q, %v", secret, err)
	}
}

func TestParseAPIScopes(t *testing.T) {
	t.Parallel()
	scopes, err := ParseAPIScopes("write, library write")
	if err != nil || strings.Join(scopes, ",") != "library,write" {
		t.Fatalf("parsed scopes = %v, %v", scopes, err)
	}
	for _, invalid := range []string{"unknown", "library,unknown", strings.Repeat("a", 257)} {
		if scopes, err := ParseAPIScopes(invalid); err == nil || scopes != nil {
			t.Fatalf("invalid scopes %q = %v, %v", invalid, scopes, err)
		}
	}
	if scopes, err := ParseAPIScopes(""); err != nil || len(scopes) != 0 {
		t.Fatalf("empty scopes = %v, %v", scopes, err)
	}
	bounded := "library" + strings.Repeat(" ", 256-len("library"))
	if scopes, err := ParseAPIScopes(bounded); err != nil || strings.Join(scopes, ",") != "library" {
		t.Fatalf("bounded scopes = %v, %v", scopes, err)
	}
}

func TestCloneAndFormatAPIKeys(t *testing.T) { //nolint:cyclop // Assertions cover copy isolation and formatting.
	t.Parallel()
	now := time.Now().Truncate(time.Second)
	keys := map[string]APIKey{
		"new": {Name: "New", Scopes: []string{"write", "library"}, CreatedAt: now.Unix(), ExpiresAt: now.Add(24 * time.Hour).Unix(), LastUsed: now.Add(-time.Hour).Unix()},
		"old": {Name: "Old", Scopes: []string{"admin"}, CreatedAt: now.Add(-48 * time.Hour).Unix(), Persistent: true},
	}
	cloned := CloneAPIKeys(keys)
	cloned["new"] = APIKey{Name: "changed"}
	old := cloned["old"]
	old.Scopes[0] = "changed"
	cloned["old"] = old
	if keys["new"].Name != "New" || keys["old"].Scopes[0] != "admin" {
		t.Fatal("cloned API keys alias source state")
	}
	views := APIKeyViews(keys)
	if len(views) != 2 || views[0].ID != "new" || views[0].Scopes != "write, library" || views[0].Expires == "Never" || views[0].LastUsed == "" {
		t.Fatalf("formatted keys = %#v", views)
	}
	if views[1].ID != "old" || views[1].Expires != "Never" || views[1].LastUsed != "" {
		t.Fatalf("persistent key view = %#v", views[1])
	}
	equal := APIKeyViews(map[string]APIKey{"b": {CreatedAt: now.Unix()}, "a": {CreatedAt: now.Unix()}})
	if len(equal) != 2 || equal[0].ID != "a" || equal[1].ID != "b" {
		t.Fatalf("equal-date key views = %#v", equal)
	}
}
