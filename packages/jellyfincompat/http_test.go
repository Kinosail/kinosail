package jellyfincompat

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type testProfile struct {
	id      string
	deleted bool
}

func TestUsersWritesOnlyVisibleProfiles(t *testing.T) {
	var result any
	Users(httptest.NewRecorder(), []testProfile{{id: "visible"}, {id: "deleted", deleted: true}}, func(profile testProfile) (map[string]any, bool) {
		return map[string]any{"id": profile.id}, !profile.deleted
	}, func(_ http.ResponseWriter, value any) { result = value })
	want := []map[string]any{{"id": "visible"}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("visible users = %v", result)
	}
}

func TestRegisterItemsMapsEveryRoute(t *testing.T) {
	mux := http.NewServeMux()
	called := make(map[string]int)
	handler := func(name string) http.HandlerFunc {
		return func(http.ResponseWriter, *http.Request) { called[name]++ }
	}
	RegisterItems(mux, ItemHandlers{
		Views: handler("views"), Items: handler("items"), Latest: handler("latest"), Persons: handler("persons"), Item: handler("item"),
		Seasons: handler("seasons"), Episodes: handler("episodes"), NextUp: handler("next-up"), Resume: handler("resume"),
	})
	for _, path := range []string{
		"/UserViews", "/Users/user/Views", "/Items", "/Users/user/Items", "/Items/Latest", "/Users/user/Items/Latest", "/Persons",
		"/Items/id", "/Users/user/Items/id", "/Shows/id/Seasons", "/Shows/id/Episodes", "/Shows/NextUp", "/UserItems/Resume",
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, response.Code)
		}
	}
	want := map[string]int{"views": 2, "items": 2, "latest": 2, "persons": 1, "item": 2, "seasons": 1, "episodes": 1, "next-up": 1, "resume": 1}
	if !reflect.DeepEqual(called, want) {
		t.Fatalf("calls = %#v, want %#v", called, want)
	}
}

func TestCreateAuthKeyRejectsInvalidAppAndCreationFailure(t *testing.T) {
	for name, test := range map[string]struct {
		target      string
		createError error
		status      int
	}{
		"invalid": {"/Auth/Keys?App=other", nil, http.StatusBadRequest},
		"failure": {"/Auth/Keys?App=seerr", errors.New("failed"), http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			calls := 0
			CreateAuthKey(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.target, nil), "viewer", func(string) (string, error) {
				calls++
				return "secret", test.createError
			}, func(string) string { return "id" }, &Grants{}, time.Now)
			if response.Code != test.status || calls != map[bool]int{true: 1, false: 0}[test.createError != nil] {
				t.Fatalf("response = %d, calls=%d", response.Code, calls)
			}
		})
	}
}

func TestCreateAndListAuthKeys(t *testing.T) {
	now := time.Unix(2_000, 0)
	grants := &Grants{}
	response := httptest.NewRecorder()
	CreateAuthKey(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/Auth/Keys?App=jellyseerr", nil), "viewer", func(app string) (string, error) {
		if app != "Seerr" {
			t.Fatalf("app = %q", app)
		}
		return "secret", nil
	}, func(secret string) string { return "id:" + secret }, grants, func() time.Time { return now })
	if response.Code != http.StatusNoContent {
		t.Fatalf("create response = %d", response.Code)
	}
	var result any
	AuthKeys(httptest.NewRecorder(), "viewer", grants, now.Add(5*time.Minute), func(_ http.ResponseWriter, value any) { result = value })
	want := map[string]any{"Items": []map[string]any{{"Id": "id:secret", "AppName": "Seerr", "AccessToken": "secret"}}, "TotalRecordCount": 1}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("auth keys = %#v", result)
	}
	AuthKeys(httptest.NewRecorder(), "viewer", grants, now.Add(5*time.Minute+time.Nanosecond), func(_ http.ResponseWriter, value any) { result = value })
	if result.(map[string]any)["TotalRecordCount"] != 0 {
		t.Fatalf("expired auth keys = %#v", result)
	}
}
