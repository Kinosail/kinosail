package identitycore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type tokenFixture struct {
	state  TokenState
	writes []string
}

func newTokenFixture(now int64) *tokenFixture {
	fixture := &tokenFixture{}
	fixture.state = TokenState{
		Sessions: map[string]Session{
			"active":  {ProfileID: "viewer", CreatedAt: now, LastSeen: now, ExpiresAt: now + 3600},
			"expired": {ProfileID: "viewer", CreatedAt: now - 7200, LastSeen: now - 3600, ExpiresAt: now - 1},
			"public":  {ProfileID: "viewer", Channel: "public", ProfileRevision: 1, CreatedAt: now, ExpiresAt: now + 3600},
			"compat":  {ProfileID: "owner", Channel: "compatibility", CreatedAt: now, ExpiresAt: now + 3600},
			"gone":    {ProfileID: "disabled", CreatedAt: now, ExpiresAt: now + 3600},
		},
		APIKeys: map[string]APIKey{
			"key":     {ProfileID: "viewer", Scopes: []string{"library:read"}},
			"old-key": {ProfileID: "viewer", ExpiresAt: now - 1},
		},
		Profiles: []Profile{{ID: "viewer", Name: "Sam", Revision: 2}, {ID: "owner", Name: "Owner", Owner: true}, {ID: "disabled", Disabled: true}},
		Use:      &AccessUse{},
		Expired:  func(session Session, now int64) bool { return session.ExpiresAt <= now },
	}
	fixture.state.Persist = func(_ context.Context, operation string, _ bool, _ int64) {
		fixture.writes = append(fixture.writes, operation)
	}
	return fixture
}

func evaluate(t *testing.T, state TokenState, key string) (TokenAccess, TokenOutcome) {
	t.Helper()
	const now = 1_800_000_000
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	access, outcome := EvaluateToken(state, request, key, now, false)
	if outcome == TokenSettle {
		access, outcome = EvaluateToken(state, request, key, now, true)
	}
	return access, outcome
}

func TestEvaluateTokenAcceptsActiveSessionsAndAPIKeys(t *testing.T) {
	t.Parallel()
	const now = 1_800_000_000
	fixture := newTokenFixture(now)
	access, outcome := evaluate(t, fixture.state, "active")
	if outcome != TokenAccepted || !access.Resolved || !access.SessionFound || access.Profile.Name != "Sam" || access.Profile.APIKey {
		t.Fatalf("active session = %#v, %v", access, outcome)
	}
	access, outcome = evaluate(t, fixture.state, "key")
	if outcome != TokenAccepted || access.SessionFound || !access.Profile.APIKey || len(access.Profile.Scopes) != 1 {
		t.Fatalf("API key = %#v, %v", access, outcome)
	}
	access, outcome = evaluate(t, fixture.state, "compat")
	if outcome != TokenAccepted || access.Profile.Owner || !access.Profile.Downloads || access.Profile.Rating != "all" {
		t.Fatalf("compatibility owner = %#v, %v", access.Profile, outcome)
	}
	if len(fixture.writes) != 0 {
		t.Fatalf("accepted tokens wrote state: %v", fixture.writes)
	}
}

func TestEvaluateTokenSettlesExpiredAndRevokedTokensUnderTheWriteLock(t *testing.T) {
	t.Parallel()
	const now = 1_800_000_000
	fixture := newTokenFixture(now)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	for key, operation := range map[string]string{"expired": "session-expire", "old-key": "api-key-expire", "public": "session-revoke"} {
		before := len(fixture.writes)
		if _, outcome := EvaluateToken(fixture.state, request, key, now, false); outcome != TokenSettle {
			t.Fatalf("%s read pass = %v, want settle", key, outcome)
		}
		if len(fixture.writes) != before {
			t.Fatalf("%s changed state during the read pass", key)
		}
		if _, outcome := EvaluateToken(fixture.state, request, key, now, true); outcome != TokenDenied {
			t.Fatalf("%s settle pass = %v, want denied", key, outcome)
		}
		if last := fixture.writes[len(fixture.writes)-1]; last != operation {
			t.Fatalf("%s wrote %q, want %q", key, last, operation)
		}
	}
	if _, found := fixture.state.Sessions["expired"]; found {
		t.Fatal("expired session kept")
	}
	if _, found := fixture.state.APIKeys["old-key"]; found {
		t.Fatal("expired API key kept")
	}
	if _, outcome := evaluate(t, fixture.state, "gone"); outcome != TokenDenied {
		t.Fatal("a disabled profile was accepted")
	}
	if _, outcome := evaluate(t, fixture.state, "missing"); outcome != TokenDenied {
		t.Fatal("an unknown token was accepted")
	}
}
