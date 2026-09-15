package scim

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSCIMUserProfileRejectsMalformedOptionalFields(t *testing.T) { //nolint:cyclop // Roles, ignored strings, and nested names are validated before projection.
	t.Parallel()
	base := scimUserPayload{Schemas: []string{scimUserSchema}, UserName: "viewer@example.com"}
	for name, modify := range map[string]func(*scimUserPayload){
		"roles":           func(payload *scimUserPayload) { payload.Roles = []json.RawMessage{json.RawMessage(`{`)} },
		"optional string": func(payload *scimUserPayload) { payload.Title = "bad\x00title" },
		"name part":       func(payload *scimUserPayload) { payload.Name.GivenName = "bad\x00name" },
	} {
		t.Run(name, func(t *testing.T) {
			payload := base
			modify(&payload)
			if _, err := payload.profile(true); err == nil {
				t.Fatal("invalid user payload accepted")
			}
		})
	}
}

func TestSCIMPatchOperationRejectsPathValueAndObjectErrors(t *testing.T) { //nolint:cyclop // Path presence, operation, value, and object fields fail independently without mutation.
	t.Parallel()
	base := scimProfileInput{UserName: "viewer@example.com", Name: "Viewer", Active: true}
	for name, operation := range map[string]scimPatchOperation{
		"blank path":           {Op: "replace", PathPresent: true, Path: " ", Value: json.RawMessage(`true`)},
		"missing value":        {Op: "replace", PathPresent: true, Path: "active"},
		"invalid operation":    {Op: "move", PathPresent: true, Path: "active", Value: json.RawMessage(`true`)},
		"unsupported path":     {Op: "replace", PathPresent: true, Path: "unknown", Value: json.RawMessage(`true`)},
		"empty object":         {Op: "replace", Value: json.RawMessage(`{}`)},
		"invalid object field": {Op: "replace", Value: json.RawMessage(`{"active":"wrong"}`)},
	} {
		t.Run(name, func(t *testing.T) {
			state := base
			if err := applySCIMPatchOperation(&state, operation); err == nil {
				t.Fatal("invalid patch operation accepted")
			}
			if !reflect.DeepEqual(state, base) {
				t.Fatalf("invalid patch mutated state: %#v", state)
			}
		})
	}
	if _, err := applySCIMPatch(viewerProfile{UserName: "viewer@example.com", Name: "Viewer"}, []scimPatchOperation{{Op: "move", PathPresent: true, Path: "active", Value: json.RawMessage(`true`)}}); err == nil {
		t.Fatal("invalid profile patch accepted")
	}
}

func TestSCIMCoreAndEnterprisePatchTypeErrors(t *testing.T) { //nolint:cyclop // Each writable scalar and complex enterprise field enforces its wire type.
	t.Parallel()
	for path, value := range map[string]string{
		"active":             `"true"`,
		"username":           `true`,
		"externalid":         `true`,
		"displayname":        `true`,
		"enterprise.manager": `{"unknown":true}`,
	} {
		state := scimProfileInput{}
		if err := applySCIMPatchField(&state, path, "replace", json.RawMessage(value)); err == nil {
			t.Fatalf("invalid %s patch accepted", path)
		}
	}
	if handled, err := applySCIMNamePatchField(&scimProfileInput{}, "other", "replace", json.RawMessage(`{}`)); handled || err != nil {
		t.Fatalf("unrelated name patch = %v, %v", handled, err)
	}
	if handled, err := applySCIMNamePatchField(&scimProfileInput{}, "name", "replace", json.RawMessage(`true`)); !handled || err == nil {
		t.Fatalf("invalid name patch = %v, %v", handled, err)
	}
	state := scimProfileInput{}
	if handled, err := applySCIMNamePatchField(&state, "name", "add", json.RawMessage(`{"formatted":"Full","givenName":"Given","familyName":"Family"}`)); !handled || err != nil || state.Formatted != "Full" || state.GivenName != "Given" || state.FamilyName != "Family" {
		t.Fatalf("added name patch = %#v, %v, %v", state, handled, err)
	}
}

func TestSCIMEmailPatchRejectsInvalidTypesAndMissingTargets(t *testing.T) { //nolint:cyclop // Collection and first-value email patch forms preserve strict types and target rules.
	t.Parallel()
	for path, value := range map[string]string{
		"emails":         `true`,
		"emails.value":   `true`,
		"emails.type":    `true`,
		"emails.primary": `"true"`,
	} {
		if err := applySCIMPatchField(&scimProfileInput{}, path, "replace", json.RawMessage(value)); err == nil {
			t.Fatalf("invalid %s patch accepted", path)
		}
	}
	if err := applySCIMPatchField(&scimProfileInput{}, "emails", "replace", json.RawMessage(`[{"value":""}]`)); err == nil {
		t.Fatal("invalid email collection accepted")
	}
	if err := applySCIMPatchField(&scimProfileInput{}, "emails.value", "replace", json.RawMessage(`""`)); err == nil {
		t.Fatal("blank email value accepted")
	}
	if err := applySCIMPatchField(&scimProfileInput{}, "emails.type", "replace", json.RawMessage(`"bad\u0000type"`)); err == nil {
		t.Fatal("invalid email type accepted")
	}
	if err := applySCIMPatchField(&scimProfileInput{}, "emails.primary", "replace", json.RawMessage(`true`)); err == nil {
		t.Fatal("email primary without a target accepted")
	}
	state := scimProfileInput{}
	if err := applySCIMPatchField(&state, "emails.value", "add", json.RawMessage(`"added@example.com"`)); err != nil || len(state.Emails) != 1 {
		t.Fatalf("added first email value = %#v, %v", state.Emails, err)
	}
	state = scimProfileInput{}
	if err := applySCIMPatchField(&state, "emails.type", "add", json.RawMessage(`"work"`)); err != nil || len(state.Emails) != 1 {
		t.Fatalf("added first email type = %#v, %v", state.Emails, err)
	}
}

func TestSCIMEmailPathParsingAndObjectFailures(t *testing.T) { //nolint:cyclop // ValuePath grammar, filter typing, object conversion, and replace targeting fail separately.
	t.Parallel()
	for _, raw := range []string{"", "emails[", "emailsx", "emails[type]", `emails[type eq true]`} {
		if _, err := parseSCIMPatchPath(raw); err == nil {
			t.Fatalf("invalid email path %q accepted", raw)
		}
	}
	if !(scimPatchPath{}).matches(scimProfileEmail{}) {
		t.Fatal("unfiltered email path did not match")
	}
	path, err := parseSCIMPatchPath(`emails[type eq "work"]`)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]json.RawMessage{
		"non-object":     json.RawMessage(`true`),
		"invalid object": json.RawMessage(`{"value":""}`),
	} {
		t.Run(name, func(t *testing.T) {
			if err := applySCIMEmailObjectPath(&scimProfileInput{}, path, "replace", value, []int{0}); err == nil {
				t.Fatal("invalid email object accepted")
			}
		})
	}
	if err := applySCIMEmailObjectPath(&scimProfileInput{}, path, "replace", json.RawMessage(`{"value":"work@example.com"}`), nil); err == nil {
		t.Fatal("replace without matching email accepted")
	}
}

func TestSCIMEmailSubattributeAndFilterEdges(t *testing.T) { //nolint:cyclop // New and existing filtered targets propagate subattribute validation without partial writes.
	t.Parallel()
	path, err := parseSCIMPatchPath(`emails[value eq "viewer@example.com"].type`)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySCIMEmailSubattributePath(&scimProfileInput{}, path, "add", json.RawMessage(`true`), nil); err == nil {
		t.Fatal("invalid new email subattribute accepted")
	}
	state := scimProfileInput{Emails: []scimProfileEmail{{Value: "viewer@example.com"}}}
	if err := applySCIMEmailSubattributePath(&state, path, "replace", json.RawMessage(`true`), []int{0}); err == nil {
		t.Fatal("invalid existing email subattribute accepted")
	}
	email := scimProfileEmail{}
	for name, call := range map[string]func() error{
		"null":            func() error { return setSCIMEmailSubattribute(&email, "value", json.RawMessage(`null`)) },
		"blank value":     func() error { return setSCIMEmailSubattribute(&email, "value", json.RawMessage(`""`)) },
		"non-string type": func() error { return setSCIMEmailSubattribute(&email, "type", json.RawMessage(`true`)) },
		"invalid type":    func() error { return setSCIMEmailSubattribute(&email, "type", json.RawMessage(`"bad\u0000type"`)) },
		"unknown":         func() error { return setSCIMEmailSubattribute(&email, "unknown", json.RawMessage(`true`)) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("invalid email subattribute accepted")
			}
		})
	}
	filtered := scimProfileEmail{}
	applySCIMEmailFilter(&filtered, scimPatchPath{filterAttribute: "value", filterValue: "viewer@example.com"})
	if filtered.Value != "viewer@example.com" {
		t.Fatalf("value filter = %#v", filtered)
	}
	if err := removeSCIMPatchField(&state, "emails[", nil); err == nil {
		t.Fatal("invalid removal path accepted")
	}
}

func TestNormalizeSCIMPrimaryWithoutPrimaryLeavesEmailsUnchanged(t *testing.T) {
	t.Parallel()
	emails := []scimProfileEmail{{Value: "one@example.com"}, {Value: "two@example.com"}}
	normalizeSCIMPrimary(emails)
	if emails[0].Primary || emails[1].Primary {
		t.Fatalf("primary added unexpectedly: %#v", emails)
	}
}

func TestSCIMCanonicalPathReturnsPlainName(t *testing.T) {
	t.Parallel()
	path := scimPatchPath{name: "name", subattribute: "givenname"}
	if path.canonical() != "name.givenname" || strings.TrimSpace(path.canonical()) == "" {
		t.Fatalf("canonical path = %q", path.canonical())
	}
}
