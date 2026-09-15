package viewing

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestViewingPageValidationAndCompletion(t *testing.T) {
	for _, test := range []struct {
		count, total int
		valid        bool
	}{
		{0, 0, true},
		{200, 200, true},
		{0, maximumViewingItems, true},
		{-1, 1, false},
		{201, 201, false},
		{0, -1, false},
		{0, maximumViewingItems + 1, false},
		{2, 1, false},
	} {
		if got := validateViewingPage(test.count, test.total); (got == nil) != test.valid {
			t.Errorf("validateViewingPage(%d, %d) = %v", test.count, test.total, got)
		}
	}
	for _, test := range []struct {
		count, total, start int
		valid               bool
	}{{0, 1000, 1000, true}, {100, 1000, 900, true}, {101, 1000, 900, false}, {0, 1001, 0, false}, {201, 201, 0, false}} {
		if got := validateViewingPlaylistPage(test.count, test.total, test.start); (got == nil) != test.valid {
			t.Errorf("validateViewingPlaylistPage(%d, %d, %d) = %v", test.count, test.total, test.start, got)
		}
	}
	for _, test := range []struct {
		count, total, start int
		complete            bool
	}{{0, 1, 0, true}, {1, 1, 0, true}, {199, 0, 0, true}, {200, 0, 0, false}, {1, 2, 0, false}} {
		if got := viewingPageComplete(test.count, test.total, test.start); got != test.complete {
			t.Errorf("viewingPageComplete(%d, %d, %d) = %t", test.count, test.total, test.start, got)
		}
	}
}

func TestNormalizeViewingActivityRejectsEveryUntrustedField(t *testing.T) { //nolint:cyclop,funlen // Each field is an independent remote trust boundary.
	valid := Activity{SourceID: "id", Kind: "movie", Title: "Title", Year: "2024", ProviderIDs: map[string]string{"tmdb": "1"}, Seconds: 2, Duration: 3}
	cases := map[string]func(*Activity){
		"missing source":   func(value *Activity) { value.SourceID = "" },
		"long source":      func(value *Activity) { value.SourceID = strings.Repeat("a", 513) },
		"missing title":    func(value *Activity) { value.Title = "" },
		"long title":       func(value *Activity) { value.Title = strings.Repeat("a", 513) },
		"long show":        func(value *Activity) { value.Show = strings.Repeat("a", 513) },
		"long path":        func(value *Activity) { value.Path = strings.Repeat("a", 4097) },
		"negative season":  func(value *Activity) { value.Season = -1 },
		"large season":     func(value *Activity) { value.Season = 10001 },
		"negative episode": func(value *Activity) { value.Episode = -1 },
		"large episode":    func(value *Activity) { value.Episode = 100001 },
		"provider count": func(value *Activity) {
			value.ProviderIDs = make(map[string]string, 17)
			for index := range 17 {
				value.ProviderIDs[string(rune('a'+index))] = "id"
			}
		},
		"bad year":          func(value *Activity) { value.Year = "x" },
		"old year":          func(value *Activity) { value.Year = "1799" },
		"future year":       func(value *Activity) { value.Year = "3001" },
		"provider name":     func(value *Activity) { value.ProviderIDs = map[string]string{strings.Repeat("a", 65): "id"} },
		"provider id":       func(value *Activity) { value.ProviderIDs = map[string]string{"tmdb": strings.Repeat("a", 513)} },
		"NaN seconds":       func(value *Activity) { value.Seconds = math.NaN() },
		"Inf seconds":       func(value *Activity) { value.Seconds = math.Inf(1) },
		"negative seconds":  func(value *Activity) { value.Seconds = -1 },
		"large seconds":     func(value *Activity) { value.Seconds = maximumViewingSeconds + 1 },
		"NaN duration":      func(value *Activity) { value.Duration = math.NaN() },
		"Inf duration":      func(value *Activity) { value.Duration = math.Inf(1) },
		"negative duration": func(value *Activity) { value.Duration = -1 },
		"large duration":    func(value *Activity) { value.Duration = maximumViewingSeconds + 1 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			activity := valid
			change(&activity)
			if _, err := normalizeViewingActivity(activity); err == nil {
				t.Fatal("invalid activity was accepted")
			}
		})
	}
	clamped, err := normalizeViewingActivity(Activity{SourceID: "id", Title: "Title", Seconds: 4, Duration: 3})
	if err != nil || clamped.Seconds != 3 {
		t.Fatalf("clamped activity = %#v, error = %v", clamped, err)
	}
	clamped.Watched, clamped.Seconds = true, 2
	clamped, err = normalizeViewingActivity(clamped)
	if err != nil || clamped.Seconds != 0 {
		t.Fatalf("watched activity = %#v, error = %v", clamped, err)
	}
	providers := make(map[string]string, 16)
	for index := range 16 {
		providers[string(rune('a'+index))] = strings.Repeat("i", 512)
	}
	boundary := Activity{SourceID: strings.Repeat("s", 512), Title: strings.Repeat("t", 512), Year: "1800", Show: strings.Repeat("h", 512), Path: strings.Repeat("p", 4096), Season: 10000, Episode: 100000, ProviderIDs: providers, Seconds: 0, Duration: maximumViewingSeconds}
	if _, err := normalizeViewingActivity(boundary); err != nil {
		t.Fatalf("lower boundary rejected: %v", err)
	}
	boundary.ProviderIDs = map[string]string{strings.Repeat("p", 64): "id"}
	if _, err := normalizeViewingActivity(boundary); err != nil {
		t.Fatalf("provider name boundary rejected: %v", err)
	}
	boundary.Year = "3000"
	boundary.Seconds = maximumViewingSeconds
	if _, err := normalizeViewingActivity(boundary); err != nil {
		t.Fatalf("upper boundary rejected: %v", err)
	}
}

func TestViewingSourceNormalizationHelpers(t *testing.T) { //nolint:cyclop,gocognit // One table covers source normalization boundaries.
	ids := normalizeProviderIDs(map[string]string{" TMDB ": " 1 ", "tvdb": "2", "IMDB": "tt3", "other": "4", "ignored": ""}) //nolint:gocritic // Deliberate whitespace proves map-key normalization.
	plex := plexProviderIDs("tmdb://1", []struct{ ID string }{{" IMDB://tt2 "}, {"tvdb://3"}, {"bad"}, {"other://4"}, {"tmdb://"}})
	if len(ids) != 3 || ids["tmdb"] != "1" || ids["tvdb"] != "2" || ids["imdb"] != "tt3" || len(plex) != 3 || plex["imdb"] != "tt2" || plex["tvdb"] != "3" {
		t.Fatalf("ids=%#v plex=%#v", ids, plex)
	}
	if parsed, err := parseViewingTime(""); err != nil || !parsed.IsZero() {
		t.Fatalf("empty time = %v, %v", parsed, err)
	}
	if parsed, err := parseViewingTime("2026-08-20T12:00:00.123Z"); err != nil || parsed.IsZero() {
		t.Fatalf("time = %v, %v", parsed, err)
	}
	if _, err := parseViewingTime("yesterday"); err == nil {
		t.Fatal("invalid time accepted")
	}
	if parsed, err := unixViewingTime(0); err != nil || !parsed.IsZero() {
		t.Fatalf("zero unix time = %v, %v", parsed, err)
	}
	if _, err := unixViewingTime(-1); err == nil {
		t.Fatal("negative unix time accepted")
	}
	if parsed, err := unixViewingTime(time.Now().Unix()); err != nil || parsed.IsZero() {
		t.Fatalf("unix time = %v, %v", parsed, err)
	}
	if _, err := unixViewingTime(32535216001); err == nil {
		t.Fatal("far-future unix time accepted")
	}
	boundaryTime := time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)
	if parsed, err := unixViewingTime(boundaryTime.Unix()); err != nil || parsed.Year() != 3000 {
		t.Fatalf("year 3000 time=%v error=%v", parsed, err)
	}
	if yearNumber(0) != "" || yearNumber(2024) != "2024" {
		t.Fatal("year formatting changed")
	}
}

func TestViewingPlaylistNamesAndMembershipsAreBounded(t *testing.T) { //nolint:cyclop,funlen,gocognit // One table covers the coupled name and membership limits.
	seen := make(map[string]int)
	first, err := uniqueViewingPlaylistName(" Queue ", seen)
	second, secondErr := uniqueViewingPlaylistName("Queue", seen)
	seen["Queue (2)"] = 1
	third, thirdErr := uniqueViewingPlaylistName(strings.Repeat("é", 32), seen)
	if err != nil || secondErr != nil || thirdErr != nil || first != "Queue" || second != "Queue (2)" || len(third) > 64 {
		t.Fatalf("names=%q/%q/%q errors=%v/%v/%v", first, second, third, err, secondErr, thirdErr)
	}
	if _, err := uniqueViewingPlaylistName("bad/name", seen); err == nil {
		t.Fatal("invalid name accepted")
	}
	collisions := map[string]int{"Queue": 1, "Queue (2)": 1, "\x00Queue": 1}
	if name, err := uniqueViewingPlaylistName("Queue", collisions); err != nil || name != "Queue (3)" {
		t.Fatalf("collision name=%q error=%v", name, err)
	}
	occupied := map[string]int{"Queue": 1}
	if name, err := uniqueViewingPlaylistName("Queue", occupied); err != nil || name != "Queue (1)" {
		t.Fatalf("occupied base name=%q error=%v", name, err)
	}
	long := strings.Repeat("é", 32)
	longSeen := map[string]int{long: 1, "\x00" + long: 1}
	if name, err := uniqueViewingPlaylistName(long, longSeen); err != nil || len(name) > 64 || !strings.HasSuffix(name, " (2)") {
		t.Fatalf("long name=%q error=%v", name, err)
	}
	exact := strings.Repeat("a", 60)
	exactSeen := map[string]int{exact: 1, "\x00" + exact: 1}
	if name, err := uniqueViewingPlaylistName(exact, exactSeen); err != nil || name != exact+" (2)" {
		t.Fatalf("exact name=%q error=%v", name, err)
	}
	exhausted := map[string]int{"Queue": 1, "\x00Queue": 1}
	for suffix := 2; suffix <= 1002; suffix++ {
		exhausted["Queue ("+strconv.Itoa(suffix)+")"] = 1
	}
	if _, err := uniqueViewingPlaylistName("Queue", exhausted); err == nil {
		t.Fatal("exhausted playlist namespace accepted")
	}
	activities := []Activity{{SourceID: "movie", Playlists: map[string]int{"Queue": 0}}}
	byID, memberships := map[string]int{"movie": 0}, maximumViewingItems
	if _, err := addViewingPlaylistItem(activities, byID, "movie", "Queue", "Plex", 1, &memberships); err != nil {
		t.Fatal(err)
	}
	if _, err := addViewingPlaylistItem(activities, byID, "movie", "Other", "Plex", 1, &memberships); err == nil {
		t.Fatal("membership limit accepted")
	}
	for _, id := range []string{"", strings.Repeat("a", 513)} {
		if _, err := addViewingPlaylistItem(nil, map[string]int{}, id, "Queue", "Plex", 0, new(int)); err == nil {
			t.Fatal("invalid item accepted")
		}
	}
	full := make([]Activity, maximumViewingItems)
	if _, err := addViewingPlaylistItem(full, map[string]int{}, "new", "Queue", "Plex", 0, new(int)); err == nil {
		t.Fatal("item limit accepted")
	}
	count, addedByID := 0, make(map[string]int)
	addedID := strings.Repeat("i", 512)
	added, err := addViewingPlaylistItem(nil, addedByID, addedID, "Queue", "Plex", 3, &count)
	if err != nil || added[0].Kind != "unsupported" || added[0].Playlists["Queue"] != 3 || count != 1 || addedByID[addedID] != 0 {
		t.Fatalf("added=%#v byID=%#v error=%v", added, addedByID, err)
	}
}
