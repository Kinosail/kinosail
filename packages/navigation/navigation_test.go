package navigation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

var allDestinations = []string{
	"home", "list", "movies", "shows", "music", "audiobooks", "books", "photos", "collections", "playlists", "unwatched", "history",
}

func TestDefaultReturnsDetachedPlayerOrder(t *testing.T) {
	t.Parallel()
	got := Default()
	if !slices.Equal(got, allDestinations) {
		t.Fatalf("Default() = %v", got)
	}
	got[0] = "changed"
	if next := Default(); next[0] != "home" {
		t.Fatalf("Default() shares storage: %v", next)
	}
}

func TestValidateAcceptsEveryBoundedDestination(t *testing.T) {
	t.Parallel()
	if err := Validate(allDestinations); err != nil {
		t.Fatalf("Validate(all) = %v", err)
	}
	for _, id := range allDestinations {
		if err := Validate([]string{id}); err != nil {
			t.Fatalf("Validate(%q) = %v", id, err)
		}
	}
}

func TestValidateRejectsInvalidNavigation(t *testing.T) {
	t.Parallel()
	for name, items := range map[string][]string{
		"empty":     nil,
		"too many":  append(append([]string(nil), allDestinations...), "extra"),
		"unknown":   {"unknown"},
		"duplicate": {"home", "home"},
	} {
		if err := Validate(items); err == nil || !strings.HasPrefix(err.Error(), "navigation ") {
			t.Errorf("%s Validate(%v) = %v", name, items, err)
		}
	}
}

func TestLinksProjectPlayerNavigation(t *testing.T) {
	t.Parallel()
	localize := func(value string) string { return "localized " + value }
	primary, more := Links([]string{"home", "movies", "shows", "music", "books"}, "movies", localize)
	wantPrimary := []Link{
		{"localized Home", "/?view=all", false},
		{"localized Movies", "/?view=movies", true},
		{"localized Shows", "/?view=shows", false},
		{"localized Music", "/?view=music", false},
	}
	if !slices.Equal(primary, wantPrimary) || !slices.Equal(more, []Link{{"localized Books", "/?view=books", false}}) {
		t.Fatalf("Links() = %#v, %#v", primary, more)
	}
	primary, more = Links([]string{"home"}, "home", localize)
	if !slices.Equal(primary, []Link{{"localized Home", "/?view=all", true}}) || more != nil {
		t.Fatalf("Links(home) = %#v, %#v", primary, more)
	}
	primary, more = Links([]string{"unknown"}, "", localize)
	if !slices.Equal(primary, []Link{{"localized ", "", true}}) || more != nil {
		t.Fatalf("Links(unknown) = %#v, %#v", primary, more)
	}
}

func TestPreferencesPutVisibleDestinationsFirst(t *testing.T) { //nolint:cyclop // One projection checks each visible and hidden position state.
	t.Parallel()
	localize := strings.ToUpper
	got := Preferences([]string{"movies", "home"}, localize)
	if len(got) != len(allDestinations) {
		t.Fatalf("len(Preferences()) = %d", len(got))
	}
	if got[0] != (Preference{"movies", "MOVIES", true, false, true}) || got[1] != (Preference{"home", "HOME", true, true, false}) {
		t.Fatalf("visible Preferences() = %#v", got[:2])
	}
	if got[2] != (Preference{ID: "list", Name: "MY LIST"}) || got[len(got)-1] != (Preference{ID: "history", Name: "HISTORY"}) {
		t.Fatalf("hidden Preferences() = %#v", got[2:])
	}

	hidden := Preferences(nil, localize)
	if len(hidden) != len(allDestinations) || hidden[0].Visible || hidden[0].CanMoveUp || hidden[0].CanMoveDown {
		t.Fatalf("Preferences(nil) = %#v", hidden)
	}
	allVisible := Preferences(allDestinations, localize)
	if len(allVisible) != len(allDestinations) || !allVisible[len(allVisible)-1].Visible || allVisible[len(allVisible)-1].CanMoveDown {
		t.Fatalf("Preferences(all) = %#v", allVisible)
	}
	unknown := Preferences([]string{"unknown"}, localize)
	if unknown[0] != (Preference{ID: "unknown", Visible: true}) {
		t.Fatalf("Preferences(unknown) = %#v", unknown)
	}
}

func TestControllerProjectsAppStateAndLocalization(t *testing.T) {
	t.Parallel()
	controller := NewController(
		func() []string { return []string{"home", "movies", "books", "photos", "history"} },
		func([]string) error { return nil },
		func(language, name string) string { return language + " " + name },
	)
	primary, more := controller.Links("all", "fr")
	if len(primary) != 4 || primary[0] != (Link{"fr Home", "/?view=all", true}) || len(more) != 1 || more[0].Name != "fr History" {
		t.Fatalf("Controller.Links() = %#v, %#v", primary, more)
	}
	preferences := controller.Preferences("es")
	if len(preferences) != len(allDestinations) || preferences[0].Name != "es Home" || preferences[len(preferences)-1].Name != "es Unwatched" {
		t.Fatalf("Controller.Preferences() = %#v", preferences)
	}
}

func TestMoveAppliesOneBoundedMove(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		items []string
		moves []string
		want  []string
	}{
		"none": {[]string{"home", "movies"}, nil, []string{"home", "movies"}},
		"up":   {[]string{"home", "movies"}, []string{"movies:up"}, []string{"movies", "home"}},
		"down": {[]string{"home", "movies"}, []string{"home:down"}, []string{"movies", "home"}},
	} {
		got, err := Move(append([]string(nil), test.items...), test.moves)
		if err != nil || !slices.Equal(got, test.want) {
			t.Errorf("%s Move() = %v, %v", name, got, err)
		}
	}
}

func TestMoveRejectsInvalidMoves(t *testing.T) {
	t.Parallel()
	for name, moves := range map[string][]string{
		"multiple":      {"movies:up", "home:down"},
		"missing colon": {"movies"},
		"bad direction": {"movies:left"},
		"extra colon":   {"movies:up:again"},
		"unknown":       {"unknown:up"},
		"before first":  {"home:up"},
		"after last":    {"movies:down"},
	} {
		got, err := Move([]string{"home", "movies"}, moves)
		if err == nil || err.Error() != "navigation move is invalid" || got != nil {
			t.Errorf("%s Move() = %v, %v", name, got, err)
		}
	}
}

func TestControllerSaveValidatesBeforeWriting(t *testing.T) {
	t.Parallel()
	writes := 0
	var saved []string
	writeErr := error(nil)
	controller := NewController(
		func() []string { return nil },
		func(items []string) error {
			writes++
			saved = append([]string(nil), items...)
			return writeErr
		},
		func(_, name string) string { return name },
	)

	request := navigationRequest("items=home&items=movies&move=movies%3Aup")
	if err := controller.Save(request); err != nil || !slices.Equal(saved, []string{"movies", "home"}) || writes != 1 {
		t.Fatalf("Save(valid) = %v, saved %v, writes %d", err, saved, writes)
	}

	writeErr = errors.New("save failed")
	if err := controller.Save(navigationRequest("items=home")); !errors.Is(err, writeErr) || writes != 2 {
		t.Fatalf("Save(write failure) = %v, writes %d", err, writes)
	}
	writeErr = nil
	malformedMedia := navigationRequest("items=home")
	malformedMedia.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset")
	wrongMedia := navigationRequest("items=home")
	wrongMedia.Header.Set("Content-Type", "text/plain")

	for name, request := range map[string]*http.Request{
		"missing media":   httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/navigation", strings.NewReader("items=home")),
		"wrong media":     wrongMedia,
		"malformed media": malformedMedia,
		"query":           formRequest("/settings/navigation?bad=1", "items=home"),
		"malformed":       navigationRequest("items=%"),
		"oversized":       navigationRequest("items=" + strings.Repeat("a", maximumFormBytes)),
		"unexpected key":  navigationRequest("items=home&bad=1"),
		"invalid move":    navigationRequest("items=home&move=home%3Aup"),
		"empty":           navigationRequest(""),
	} {
		before := writes
		if err := controller.Save(request); err == nil || writes != before {
			t.Errorf("%s Save() = %v, writes %d", name, err, writes)
		}
	}
}

func TestControllerHandlerUsesAppFailureAndRedirects(t *testing.T) {
	t.Parallel()
	writes := 0
	controller := NewController(
		func() []string { return nil },
		func([]string) error { writes++; return nil },
		func(_, name string) string { return name },
	)
	failures := 0
	handler := controller.Handler(func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		failures++
		if message == "" || status != http.StatusBadRequest {
			t.Errorf("failure = %q, %d", message, status)
		}
		writer.WriteHeader(status)
	})

	valid := httptest.NewRecorder()
	handler.ServeHTTP(valid, navigationRequest("items=home"))
	if valid.Code != http.StatusSeeOther || valid.Header().Get("Location") != "/settings#navigation" || writes != 1 || failures != 0 {
		t.Fatalf("valid handler = %d, %q, writes %d, failures %d", valid.Code, valid.Header().Get("Location"), writes, failures)
	}

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/navigation", nil))
	if invalid.Code != http.StatusBadRequest || writes != 1 || failures != 1 {
		t.Fatalf("invalid handler = %d, writes %d, failures %d", invalid.Code, writes, failures)
	}
}

func navigationRequest(form string) *http.Request {
	return formRequest("/settings/navigation", form)
}

func formRequest(target, form string) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, target, strings.NewReader(form))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}
