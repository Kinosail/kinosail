package identitycore

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProfileAllowed(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		profile Profile
		public  bool
		now     time.Time
		want    bool
	}{
		{name: "owner", profile: Profile{Owner: true}, public: true, want: true},
		{name: "remote denied", profile: Profile{}, public: true, want: false},
		{name: "no schedule", profile: Profile{Remote: true}, public: true, want: true},
		{name: "day inside", profile: Profile{AccessStart: "09:00", AccessEnd: "17:00"}, now: day.Add(10 * time.Hour), want: true},
		{name: "day end excluded", profile: Profile{AccessStart: "09:00", AccessEnd: "17:00"}, now: day.Add(17 * time.Hour), want: false},
		{name: "overnight late", profile: Profile{AccessStart: "21:00", AccessEnd: "06:00"}, now: day.Add(23 * time.Hour), want: true},
		{name: "overnight early", profile: Profile{AccessStart: "21:00", AccessEnd: "06:00"}, now: day.Add(5 * time.Hour), want: true},
		{name: "overnight outside", profile: Profile{AccessStart: "21:00", AccessEnd: "06:00"}, now: day.Add(12 * time.Hour), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.profile.Allowed(test.public, test.now); got != test.want {
				t.Fatalf("Allowed() = %v; want %v", got, test.want)
			}
		})
	}
}

func TestProfilePolicyFromRequest(t *testing.T) {
	t.Parallel()
	form := url.Values{
		"downloads": {"true"}, "transcode": {"true"}, "remote": {"true"}, "rating": {"teen"},
		"libraries": {" Shows, Movies,Shows, "}, "start": {" 09:00 "}, "end": {" 17:00 "},
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	want := ProfilePolicy{Downloads: true, Transcode: true, Remote: true, Rating: "teen", Libraries: []string{"Movies", "Shows"}, AccessStart: "09:00", AccessEnd: "17:00"}
	if got := ProfilePolicyFromRequest(request); !reflect.DeepEqual(got, want) {
		t.Fatalf("ProfilePolicyFromRequest() = %#v; want %#v", got, want)
	}
}

func TestProfilePolicyValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		policy ProfilePolicy
		valid  bool
	}{
		{name: "empty", valid: true},
		{name: "valid", policy: ProfilePolicy{AccessStart: "09:00", AccessEnd: "17:00"}, valid: true},
		{name: "missing end", policy: ProfilePolicy{AccessStart: "09:00"}},
		{name: "missing start", policy: ProfilePolicy{AccessEnd: "17:00"}},
		{name: "bad start", policy: ProfilePolicy{AccessStart: "bad", AccessEnd: "17:00"}},
		{name: "bad end", policy: ProfilePolicy{AccessStart: "09:00", AccessEnd: "bad"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.policy.Valid(); (got == nil) != test.valid {
				t.Fatalf("Valid() = %v; valid = %v", got, test.valid)
			}
		})
	}
}

func TestApplyProfilePolicy(t *testing.T) {
	t.Parallel()
	for rating, want := range map[string]string{"teen": "teen", "all": "all", "unknown": "family"} {
		profile := Profile{}
		libraries := []string{"movies"}
		policy := ProfilePolicy{Downloads: true, Transcode: true, Remote: true, Rating: rating, Libraries: libraries, AccessStart: "09:00", AccessEnd: "17:00"}
		ApplyProfilePolicy(&profile, policy)
		libraries[0] = "changed"
		if !profile.Downloads || !profile.Transcode || !profile.Remote || profile.Rating != want || profile.Libraries[0] != "movies" || profile.AccessStart != "09:00" || profile.AccessEnd != "17:00" {
			t.Fatalf("ApplyProfilePolicy(%q) = %#v", rating, profile)
		}
	}
}
