package downloads

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestRequestAccessAppliesPlayerDownloadPolicy(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/downloads", nil)
	for _, test := range []struct {
		name                        string
		profile                     identitycore.Profile
		found                       bool
		profileAllowed, itemAllowed bool
	}{
		{"owner", identitycore.Profile{ID: "owner", Owner: true}, true, true, true},
		{"viewer", identitycore.Profile{ID: "viewer", Downloads: true}, true, true, true},
		{"denied", identitycore.Profile{ID: "viewer"}, true, false, false},
		{"scoped key", identitycore.Profile{ID: "key", Downloads: true, APIKey: true, Scopes: []string{"download"}}, true, true, true},
		{"unscoped key", identitycore.Profile{ID: "key", Downloads: true, APIKey: true}, true, false, false},
		{"missing item", identitycore.Profile{ID: "viewer", Downloads: true}, false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			access := NewAccess(func(got *http.Request) identitycore.Profile {
				if got != request {
					t.Fatal("profile request was replaced")
				}
				return test.profile
			}, func(got *http.Request, id string) (library.Item, bool) {
				if got != request || id != "film" {
					t.Fatalf("item input = %p %q", got, id)
				}
				return library.Item{ID: id}, test.found
			})
			profileID, profileAllowed := access.Profile(request)
			itemProfileID, item, itemAllowed := access.Item(request, "film")
			if profileID != test.profile.ID || itemProfileID != test.profile.ID || item.ID != "film" || profileAllowed != test.profileAllowed || itemAllowed != test.itemAllowed {
				t.Fatalf("profile = %q/%t; item = %q %#v/%t", profileID, profileAllowed, itemProfileID, item, itemAllowed)
			}
		})
	}
}

func TestRequestAccessFailsClosedWithoutBindings(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/downloads", nil)
	for _, test := range []struct {
		access         Access
		profileID      string
		profileAllowed bool
	}{
		{NewAccess(nil, nil), "", false},
		{NewAccess(func(*http.Request) identitycore.Profile { return identitycore.Profile{ID: "viewer", Downloads: true} }, nil), "viewer", true},
	} {
		if profileID, allowed := test.access.Profile(request); allowed != test.profileAllowed || profileID != test.profileID {
			t.Fatalf("profile = %q/%t", profileID, allowed)
		}
		if profileID, item, allowed := test.access.Item(request, "film"); allowed || item.ID != "" || profileID != "" {
			t.Fatalf("item = %q %#v/%t", profileID, item, allowed)
		}
	}
}
