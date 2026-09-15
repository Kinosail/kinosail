package scim

import (
	"testing"
)

func TestSCIMSchemaSelectionAndEntityTagEdges(t *testing.T) { //nolint:cyclop // Schema, mandatory projection, and entity-tag grammar have independent boundaries.
	t.Parallel()
	if err := validateSCIMSchema(nil, scimUserSchema); err == nil {
		t.Fatal("missing SCIM schema accepted")
	}
	selection := scimSelection{excluded: map[string]bool{"displayname": true}}
	if !selection.includes("id") || selection.includes("displayName") {
		t.Fatal("mandatory or excluded SCIM selection mismatch")
	}
	if !validSCIMETagHeader("*") {
		t.Fatal("wildcard entity tag rejected")
	}
	if validSCIMETagHeader(`"bad value"`) {
		t.Fatal("entity tag with prohibited space accepted")
	}
	if !ETagMatches("*", `W/"1"`) {
		t.Fatal("wildcard entity tag did not match")
	}
}
