package servertest

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func assertExternalSettingsLocked(t *testing.T, settings, locked, lockedMFA, lockedJellyfin, lockedSCIM, afterRejectedChanges *httptest.ResponseRecorder, jellyfin bool) {
	t.Helper()
	expected := []string{`"name":"Environment Home"`, `"requireMfa":false`}
	if jellyfin {
		expected = append(expected, `"jellyfinCompatibility":false`)
	}
	AssertAPIBody(t, settings, http.StatusOK, expected...)
	for _, response := range []*httptest.ResponseRecorder{locked, lockedMFA, lockedJellyfin, lockedSCIM} {
		AssertAPIBody(t, response, http.StatusConflict)
	}
	AssertAPIBody(t, afterRejectedChanges, http.StatusOK, expected[1:]...)
}

func (fixture ConfigurationHTTP[Source, S]) withoutJellyfinOrDLNA(expected []string) []string {
	if !fixture.NoJellyfinOrDLNA {
		return expected
	}
	return slices.DeleteFunc(expected, func(value string) bool {
		return strings.Contains(strings.ToLower(value), "jellyfin") || strings.Contains(strings.ToLower(value), "dlna")
	})
}
