package servertest

import (
	"net/http"
	"net/http/httptest"
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
