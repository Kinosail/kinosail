package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The departing watch page may never have saved its UUID before Mark watched.
// This is the authenticated sequence observed in the WebKit Owner journey.
func TestWatchedHTTPClosesUnregisteredCurrentPage(t *testing.T) {
	for _, adapter := range []string{"web", "API"} {
		t.Run(adapter, func(t *testing.T) {
			fixture := newWatchedSessionHTTP(t)
			fixture.progress(t, "API", "previous-watch-page", "4", "12", http.StatusOK)
			page := fixture.request(t, http.MethodGet, "/watch/"+fixture.id, "", "", http.StatusOK)
			session := watchedPageSession(t, string(page))
			if !strings.Contains(string(page), `name="session" value="`+session+`"`) {
				t.Error("watched form omitted its current playback session")
			}
			fixture.markCurrent(t, true, session)
			before := fixture.state(t)
			fixture.restart(t)
			fixture.unchanged(t, before)
			fixture.progress(t, adapter, session, "1", "0.115", fixture.rejected(adapter))
			fixture.unchanged(t, before)
			fixture.markCurrent(t, false, session)
			fixture.progress(t, adapter, session, "2", "5", fixture.success(adapter))
			if state := fixture.state(t); state.Watched || state.Seconds != 5 {
				t.Fatal("explicit Mark unwatched did not permit resumed playback")
			}
			fixture.markCurrent(t, true, session)
			fixture.progress(t, adapter, "new-explicit-play", "1", "7", fixture.success(adapter))
			if state := fixture.state(t); state.Watched || state.Seconds != 7 {
				t.Fatal("a new playback session could not save progress")
			}
		})
	}
}

func TestWatchedHTTPRejectsAmbiguousCurrentPageWithoutMutation(t *testing.T) {
	for name, body := range map[string]string{
		"missing watched":    "session=current-page",
		"malformed watched":  "watched=sometimes&session=current-page",
		"duplicate watched":  "watched=true&watched=false&session=current-page",
		"empty session":      "watched=true&session=",
		"short session":      "watched=true&session=short",
		"oversized session":  "watched=true&session=" + strings.Repeat("s", 65),
		"malformed session":  "watched=true&session=current%2Fpage",
		"control session":    "watched=true&session=current%00page",
		"unicode session":    "watched=true&session=current%C3%A9page",
		"duplicate session":  "watched=true&session=current-page&session=another-page",
		"unknown field":      "watched=true&session=current-page&unexpected=1",
		"oversized body":     "watched=true&session=current-page" + strings.Repeat("&", 1<<20),
		"malformed encoding": "watched=true&session=%zz",
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newWatchedSessionHTTP(t)
			fixture.progress(t, "API", "previous-watch-page", "4", "12", http.StatusOK)
			before := fixture.state(t)
			status := http.StatusBadRequest
			if name == "oversized body" {
				status = http.StatusRequestEntityTooLarge // The existing whole-request admission runs before CSRF/form parsing.
			}
			fixture.request(t, http.MethodPost, "/watched/"+fixture.id, "application/x-www-form-urlencoded", body, status)
			fixture.unchanged(t, before)
			fixture.restart(t)
			fixture.unchanged(t, before)
		})
	}
	for _, query := range []string{"?session=current-page", "?watched=false"} {
		t.Run("conflicting query "+query, func(t *testing.T) {
			fixture := newWatchedSessionHTTP(t)
			before := fixture.state(t)
			fixture.request(t, http.MethodPost, "/watched/"+fixture.id+query, "application/x-www-form-urlencoded", "watched=true&session=current-page", http.StatusBadRequest)
			fixture.unchanged(t, before)
		})
	}
}

func TestWatchedHTTPAcceptsBoundedCurrentPageIdentity(t *testing.T) {
	for _, session := range []string{"Page_1-a", strings.Repeat("s", 64)} {
		t.Run(session, func(t *testing.T) {
			fixture := newWatchedSessionHTTP(t)
			fixture.markCurrent(t, true, session)
			before := fixture.state(t)
			fixture.progress(t, "API", session, "1", "1", http.StatusConflict)
			fixture.unchanged(t, before)
		})
	}
}

func watchedPageSession(t *testing.T, page string) string {
	t.Helper()
	_, remaining, found := strings.Cut(page, `data-playback-session="`)
	if !found {
		t.Fatal("public watch page omitted its playback session")
	}
	session, _, found := strings.Cut(remaining, `"`)
	if !found || len(session) < 8 || len(session) > 64 {
		t.Fatal("public watch page returned an invalid session")
	}
	return session
}

func (fixture *watchedSessionHTTP) markCurrent(t *testing.T, watched bool, session string) {
	t.Helper()
	value := "false"
	if watched {
		value = "true"
	}
	form := url.Values{"watched": {value}, "session": {session}}
	fixture.request(t, http.MethodPost, "/watched/"+fixture.id, "application/x-www-form-urlencoded", form.Encode(), http.StatusSeeOther)
}
