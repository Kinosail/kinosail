package scim

import (
	"encoding/json"
	"testing"
)

func TestSCIMFilterAndEmailSelectorBranches(t *testing.T) { //nolint:gocognit,cyclop // Direct unit coverage enumerates filter and multi-value selector branches.
	profile := viewerProfile{UserName: "viewer@example.com", ExternalID: "directory-id", Name: "Viewer Name", NameParts: scimProfileName{Formatted: "Viewer Name", GivenName: "Viewer", FamilyName: "Name"}}
	for attribute, expected := range map[string]string{"username": profile.UserName, "externalid": profile.ExternalID, "displayname": profile.Name, "name.formatted": "Viewer Name", "name.givenname": "Viewer", "name.familyname": "Name", "unknown": ""} {
		if actual := scimFilterAttribute(profile, attribute); actual != expected {
			t.Fatalf("SCIM filter attribute %s = %q, want %q", attribute, actual, expected)
		}
	}
	for exact, cases := range map[bool][]struct {
		operator, actual, expected string
		want                       bool
	}{
		true:  {{"eq", "Directory-ID", "directory-id", false}, {"sw", "directory-id", "directory", true}, {"co", "directory-id", "rect", true}},
		false: {{"eq", "Viewer", "viewer", true}, {"sw", "Viewer Name", "view", true}, {"co", "Viewer Name", "ame", true}},
	} {
		for _, test := range cases {
			if actual := scimFilterMatch(test.actual, test.expected, test.operator, exact); actual != test.want {
				t.Fatalf("SCIM filter match exact=%v %#v = %v", exact, test, actual)
			}
		}
	}
	state := scimProfileInput{Emails: []scimProfileEmail{{Value: "work@example.com", Type: "work"}}}
	path, err := parseSCIMPatchPath(`emails[type eq "work"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySCIMEmailPath(&state, path, "replace", json.RawMessage(`{"value":"updated@example.com"}`)); err != nil {
		t.Fatal(err)
	}
	path, err = parseSCIMPatchPath(`emails[type eq "other"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySCIMEmailPath(&state, path, "add", json.RawMessage(`{"value":"other@example.com"}`)); err != nil {
		t.Fatal(err)
	}
	path, err = parseSCIMPatchPath(`emails[primary eq true].value`)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySCIMEmailPath(&state, path, "add", json.RawMessage(`"primary@example.com"`)); err != nil {
		t.Fatal(err)
	}
	for _, rawPath := range []string{`emails[type eq "other"].type`, `emails[primary eq true].primary`} {
		if err := removeSCIMPatchField(&state, rawPath, nil); err != nil {
			t.Fatal(err)
		}
	}
	if (scimPatchPath{filterAttribute: "unknown", hasFilter: true}).matches(scimProfileEmail{}) {
		t.Fatal("unknown SCIM email filter matched")
	}
}
