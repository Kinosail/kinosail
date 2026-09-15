package jellyfincompat

import (
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestIdentifierGenerationAndValidation(t *testing.T) { //nolint:cyclop // One boundary table verifies every permitted character range.
	id, err := IDFrom(strings.NewReader("0123456789abcdef"))
	if err != nil || id != "30313233343536373839616263646566" || !ValidID(id) {
		t.Fatalf("identifier = %q, error = %v", id, err)
	}
	if id, err = IDFrom(strings.NewReader("short")); err == nil || id != "" {
		t.Fatalf("short entropy identifier = %q, error = %v", id, err)
	}
	if id, err = NewID(); err != nil || len(id) != 32 || !ValidID(id) {
		t.Fatalf("random identifier = %q, error = %v", id, err)
	}
	for _, valid := range []string{"-", "0", "9", "A", "Z", "a", "z", strings.Repeat("a", 128)} {
		if !ValidID(valid) {
			t.Errorf("valid identifier rejected: %q", valid)
		}
	}
	for _, invalid := range []string{"", "/", ":", "@", "[", "_", "`", "{", strings.Repeat("a", 129)} {
		if ValidID(invalid) {
			t.Errorf("invalid identifier accepted: %q", invalid)
		}
	}
}

func TestEnsureIDValidatesBeforeGeneration(t *testing.T) { //nolint:cyclop // One lifecycle proves validation and generation have no mixed side effects.
	calls := 0
	generate := func() (string, error) {
		calls++
		return "generated", nil
	}
	if id, changed, err := EnsureID("existing", generate); err != nil || changed || id != "existing" || calls != 0 {
		t.Fatalf("existing identifier = %q, %t, %v, calls %d", id, changed, err, calls)
	}
	if id, changed, err := EnsureID("bad_id", generate); err == nil || changed || id != "" || calls != 0 {
		t.Fatalf("invalid identifier = %q, %t, %v, calls %d", id, changed, err, calls)
	}
	if id, changed, err := EnsureID("", generate); err != nil || !changed || id != "generated" || calls != 1 {
		t.Fatalf("generated identifier = %q, %t, %v, calls %d", id, changed, err, calls)
	}
	wantErr := errors.New("entropy unavailable")
	if id, changed, err := EnsureID("", func() (string, error) { return "", wantErr }); !errors.Is(err, wantErr) || changed || id != "" {
		t.Fatalf("failed identifier = %q, %t, %v", id, changed, err)
	}
}

func TestLibraryRoutesAreFreshAndComplete(t *testing.T) {
	routes := LibraryRoutes()
	if len(routes) != 18 || routes[0] != "GET /System/Info" || routes[17] != "GET /Items/{id}/Images/{type}/{index}" {
		t.Fatalf("library routes = %v", routes)
	}
	routes[0] = "changed"
	if LibraryRoutes()[0] != "GET /System/Info" {
		t.Fatal("library routes shared mutable state")
	}
}

func TestAppAcceptsOneSupportedSeerrName(t *testing.T) {
	for name, values := range map[string]url.Values{
		"seerr":      {"App": {" SeErR "}},
		"jellyseerr": {"app": {"jellyseerr"}},
	} {
		if app, ok := App(values); !ok || app != "Seerr" {
			t.Errorf("%s app = %q, %t", name, app, ok)
		}
	}
	for name, values := range map[string]url.Values{
		"missing":     {},
		"unsupported": {"App": {"other"}},
		"duplicate":   {"App": {"seerr"}, "app": {"seerr"}},
		"long":        {"App": {strings.Repeat("a", len("jellyseerr")+1)}},
	} {
		if app, ok := App(values); ok || app != "" {
			t.Errorf("%s app = %q, %t", name, app, ok)
		}
	}
}

func TestProviderIDsUseOnlyKnownNonemptyNames(t *testing.T) {
	got := ProviderIDs(map[string]string{
		"tmdb": "1", "TheMovieDB": "2", "IMDB": "3", "tvdb": "4", "AniDB": "5", "unknown": "6", "blank": " ",
	})
	want := map[string]string{"Tmdb": "1", "TheMovieDb": "2", "Imdb": "3", "Tvdb": "4", "AniDB": "5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("provider IDs = %v, want %v", got, want)
	}
	if got := ShowProviderIDs([]library.Item{{}, {ShowProviderIDs: map[string]string{"tmdb": "7"}}, {ShowProviderIDs: map[string]string{"tmdb": "8"}}}); !reflect.DeepEqual(got, map[string]string{"Tmdb": "7"}) {
		t.Fatalf("show provider IDs = %v", got)
	}
	if got := ShowProviderIDs(nil); len(got) != 0 || got == nil {
		t.Fatalf("empty show provider IDs = %#v", got)
	}
	if got := ProviderIDs(map[string]string{"tmdb": " ", "imdb": "kept"}); !reflect.DeepEqual(got, map[string]string{"Imdb": "kept"}) {
		t.Fatalf("blank provider did not skip to next entry: %v", got)
	}
}

func TestItemIDsValidateOneBoundedQuery(t *testing.T) { //nolint:cyclop // One boundary table verifies every accepted and rejected query shape.
	normalize := func(value string) string { return "id:" + value }
	if ids, err := ItemIDs(nil, normalize); err != nil || ids != nil {
		t.Fatalf("missing IDs = %v, %v", ids, err)
	}
	ids, err := ItemIDs(url.Values{"IDs": {" first ,second "}}, normalize)
	if err != nil || !reflect.DeepEqual(ids, map[string]bool{"id:first": true, "id:second": true}) {
		t.Fatalf("valid IDs = %v, %v", ids, err)
	}
	tooMany := make([]string, 101)
	for index := range tooMany {
		tooMany[index] = "id"
	}
	maximum := make([]string, 100)
	for index := range maximum {
		maximum[index] = "id"
	}
	if ids, err := ItemIDs(url.Values{"ids": {strings.Join(maximum, ",")}}, normalize); err != nil || len(ids) != 1 {
		t.Fatalf("maximum IDs = %v, %v", ids, err)
	}
	maximumPart := strings.Repeat("a", 64)
	if ids, err := ItemIDs(url.Values{"ids": {maximumPart}}, normalize); err != nil || !ids["id:"+maximumPart] {
		t.Fatalf("maximum-length ID = %v, %v", ids, err)
	}
	for name, values := range map[string]url.Values{
		"duplicate": {"ids": {"one"}, "IDS": {"two"}},
		"empty":     {"ids": {""}},
		"long":      {"ids": {strings.Repeat("a", 65)}},
		"control":   {"ids": {"bad\nvalue"}},
		"DEL":       {"ids": {"a\x7f"}},
		"NUL first": {"ids": {"\x00a"}},
		"many":      {"ids": {strings.Join(tooMany, ",")}},
	} {
		if ids, err := ItemIDs(values, normalize); err == nil || ids != nil {
			t.Errorf("%s IDs = %v, %v", name, ids, err)
		}
	}
}

func TestInvalidControlBoundaries(t *testing.T) {
	for value, want := range map[rune]bool{0x1f: true, 0x20: false, 0x7f: true, 0x80: false} {
		if got := invalidControl(value); got != want {
			t.Errorf("invalidControl(%#x) = %t, want %t", value, got, want)
		}
	}
}

func TestGrantsSortScopeExpireAndCleanInvalidState(t *testing.T) {
	now := time.Unix(1_000, 0)
	grants := &Grants{}
	grants.Store("viewer", "Zulu", "z-secret", "z-id", now.Add(time.Minute))
	grants.Store("viewer", "Alpha", "a-secret", "a-id", now)
	grants.Store("other", "Other", "o-secret", "o-id", now.Add(time.Minute))
	grants.Store("viewer", "Expired", "e-secret", "e-id", now.Add(-time.Nanosecond))
	grants.values.Store("invalid", "invalid")
	items := grants.Items("viewer", now)
	if len(items) != 2 || items[0]["AppName"] != "Alpha" || items[1]["AppName"] != "Zulu" || items[0]["Id"] != "a-id" || items[0]["AccessToken"] != "a-secret" {
		t.Fatalf("grant items = %v", items)
	}
	if _, found := grants.values.Load("invalid"); found {
		t.Fatal("invalid grant state was retained")
	}
	if expired := grants.Items("viewer", now.Add(time.Minute+time.Nanosecond)); len(expired) != 0 {
		t.Fatalf("expired grants = %v", expired)
	}
}
