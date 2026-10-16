package servertest

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const loginChurnClients = 4096

func AssertLoginLimitsSurviveClientChurn(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()

	handler := fixture.NewHandler("", t.TempDir(), true)
	_ = fixture.SignIn(t, handler, "/setup", "name=Owner&password=correct-password")
	exhaustLoginAccount(t, handler, "198.51.100.10", "Owner")
	exhaustLoginAccount(t, handler, "198.51.100.11", "ghost")
	for client := range loginChurnClients {
		remote := "10." + strconv.Itoa(client>>8) + "." + strconv.Itoa(client&0xff) + ".1"
		if code := passwordLogin(t, handler, remote, "ghost"); code != http.StatusTooManyRequests {
			t.Fatalf("churn login %d = %d; want throttled", client, code)
		}
	}
	if code := passwordLogin(t, handler, "198.51.100.10", "Owner"); code != http.StatusTooManyRequests {
		t.Fatalf("exhausted Owner login after client churn = %d; want throttled", code)
	}
}

func AssertLoginNameFloodDoesNotLockOutOtherClients(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()

	handler := fixture.NewHandler("", t.TempDir(), true)
	_ = fixture.SignIn(t, handler, "/setup", "name=Owner&password=correct-password")
	for attempt := range 30 + loginChurnClients {
		want := http.StatusUnauthorized
		if attempt >= 30 {
			want = http.StatusTooManyRequests
		}
		if code := passwordLogin(t, handler, "198.51.100.20", "flood-"+strconv.Itoa(attempt)); code != want {
			t.Fatalf("flood login %d = %d; want %d", attempt, code, want)
		}
	}
	if code := passwordLogin(t, handler, "198.51.100.21", "Owner"); code != http.StatusUnauthorized {
		t.Fatalf("other client login after name flood = %d; want password check", code)
	}
}

func exhaustLoginAccount(t *testing.T, handler http.Handler, remote, name string) {
	t.Helper()
	for attempt := range 10 {
		if code := passwordLogin(t, handler, remote, name); code != http.StatusUnauthorized {
			t.Fatalf("%s login %d = %d; want password check", name, attempt, code)
		}
	}
	if code := passwordLogin(t, handler, remote, name); code != http.StatusTooManyRequests {
		t.Fatalf("%s login after its limit = %d; want throttled", name, code)
	}
}

func passwordLogin(t *testing.T, handler http.Handler, remote, name string) int {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name="+name+"&password=wrong-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = remote + ":1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response.Code
}
