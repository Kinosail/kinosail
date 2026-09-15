package scim

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSCIMPatchValuePathsAndRemoval(t *testing.T) { //nolint:gocognit,cyclop // Direct unit coverage enumerates the SCIM valuePath state transitions.
	state := scimProfileInput{UserName: "viewer@example.com", Name: "Viewer", Active: true, Emails: []scimProfileEmail{{Value: "home@example.com", Type: "home"}}}
	path, err := parseSCIMPatchPath(`emails[type eq "work"].value`)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySCIMEmailPath(&state, path, "add", json.RawMessage(`"work@example.com"`)); err != nil {
		t.Fatal(err)
	}
	if len(state.Emails) != 2 || state.Emails[1].Type != "work" {
		t.Fatalf("added email = %#v", state.Emails)
	}
	path, err = parseSCIMPatchPath(`emails[type eq "work"].primary`)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySCIMEmailPath(&state, path, "replace", json.RawMessage("true")); err != nil {
		t.Fatal(err)
	}
	path, err = parseSCIMPatchPath(`emails[primary eq true].type`)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySCIMEmailPath(&state, path, "replace", json.RawMessage(`"preferred"`)); err != nil {
		t.Fatal(err)
	}
	if _, err := parseSCIMPatchPath(`emails[value eq "work@example.com"]`); err != nil {
		t.Fatal(err)
	}
	if err := removeSCIMPatchField(&state, `emails[value eq "work@example.com"]`, nil); err != nil {
		t.Fatal(err)
	}
	if len(state.Emails) != 1 || state.Emails[0].Type != "home" {
		t.Fatalf("removed email = %#v", state.Emails)
	}

	missing, err := parseSCIMPatchPath(`emails[type eq "missing"].value`)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySCIMEmailPath(&state, missing, "replace", json.RawMessage(`"missing@example.com"`)); err == nil {
		t.Fatal("replace without an email target succeeded")
	}
	if err := removeSCIMPatchField(&state, `emails[type eq "missing"]`, nil); err == nil {
		t.Fatal("remove without an email target succeeded")
	}

	for _, rawPath := range []string{"emails[unknown eq \"x\"]", "emails[type ne \"x\"]", "emails[type eq \"x\"", "emails.bad", "emails[primary eq nope]"} {
		if _, err := parseSCIMPatchPath(rawPath); err == nil {
			t.Fatalf("invalid SCIM email path %q was accepted", rawPath)
		}
	}
}

func TestSCIMPatchFieldTypesAndRemovals(t *testing.T) {
	state := scimProfileInput{UserName: "old@example.com", Name: "Old", Formatted: "Old", GivenName: "Old", FamilyName: "Viewer", ExternalID: "old-id", Active: true, Emails: []scimProfileEmail{{Value: "old@example.com", Type: "home", Primary: true}}}
	assertSCIMPatchFieldMatrix(t, &state)
	assertSCIMNullPatchFields(t, &state)
	assertSCIMPatchPrecedenceAndPrimary(t)
}

func TestSCIMValidationAndWireTypes(t *testing.T) { //nolint:gocognit,cyclop,funlen // Direct unit coverage keeps persistence and wire validation boundaries explicit.
	if _, err := scimString(strings.Repeat("x", 5), 4, false, "field"); err == nil {
		t.Fatal("oversized SCIM value was accepted")
	}
	for _, value := range []string{"bad\tvalue", "bad\x7fvalue", "\tbad"} {
		if _, err := scimString(value, 32, false, "field"); err == nil {
			t.Fatalf("SCIM control character was accepted: %q", value)
		}
	}
	var name scimName
	if err := json.Unmarshal([]byte("null"), &name); err != nil || name != (scimName{}) {
		t.Fatal("null SCIM name did not clear the optional value")
	}
	var email scimEmail
	if err := json.Unmarshal([]byte("null"), &email); err == nil {
		t.Fatal("null SCIM email was accepted")
	}
	filter, err := scimFilter("externalId pr")
	if err != nil || !filter(viewerProfile{ExternalID: "directory-id"}) || filter(viewerProfile{}) {
		t.Fatalf("SCIM presence filter matched=%v, err=%v", filter != nil && filter(viewerProfile{ExternalID: "directory-id"}), err)
	}
	if _, err := scimFilter("displayName ne \"Viewer\""); err == nil {
		t.Fatal("unsupported SCIM filter operator was accepted")
	}
	filter, err = scimFilter(scimUserSchema + `:userName eq "viewer@example.com"`)
	if err != nil || !filter(viewerProfile{UserName: "viewer@example.com"}) {
		t.Fatalf("schema-qualified SCIM filter matched=%v err=%v", err == nil && filter != nil && filter(viewerProfile{UserName: "viewer@example.com"}), err)
	}
	if (&scimNoTargetError{detail: "missing"}).Error() != "missing" {
		t.Fatal("SCIM no-target error lost its detail")
	}
	for err, status := range map[error]int{ErrProfileNotFound: http.StatusNotFound, ErrConflict: http.StatusConflict, ErrSetupRequired: http.StatusServiceUnavailable, ErrPrecondition: http.StatusPreconditionFailed, errors.New("unexpected"): http.StatusInternalServerError} {
		response := httptest.NewRecorder()
		writeSCIMStoreError(response, err)
		if response.Code != status {
			t.Fatalf("SCIM store error %v = %d, want %d", err, response.Code, status)
		}
	}
	response := httptest.NewRecorder()
	writeSCIMValidationError(response, &scimNoTargetError{detail: "missing"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("SCIM no-target validation = %d", response.Code)
	}
}

func TestSCIMNormalizationRejectsInvalidFields(t *testing.T) {
	base := func() scimProfileInput {
		return scimProfileInput{UserName: "viewer@example.com", Name: "Viewer"}
	}
	for _, testCase := range []struct {
		name   string
		modify func(*scimProfileInput)
	}{
		{"display name", func(input *scimProfileInput) { input.Name = "" }},
		{"formatted name", func(input *scimProfileInput) { input.Formatted = "bad\x00value" }},
		{"given name", func(input *scimProfileInput) { input.GivenName = "bad\x00value" }},
		{"family name", func(input *scimProfileInput) { input.FamilyName = "bad\x00value" }},
		{"external id", func(input *scimProfileInput) { input.ExternalID = "bad\x00value" }},
		{"too many emails", func(input *scimProfileInput) { input.Emails = make([]scimProfileEmail, 17) }},
		{"email value", func(input *scimProfileInput) { input.Emails = []scimProfileEmail{{Value: ""}} }},
		{"email type", func(input *scimProfileInput) {
			input.Emails = []scimProfileEmail{{Value: "viewer@example.com", Type: "bad\x00value"}}
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			input := base()
			testCase.modify(&input)
			if _, err := NormalizeProfileInput(input); err == nil {
				t.Fatal("invalid SCIM field was accepted")
			}
		})
	}
	if (&scimValidationError{detail: "invalid"}).Error() != "invalid" {
		t.Fatal("SCIM validation error lost its detail")
	}
	if _, err := NormalizeProfileInput(scimProfileInput{UserName: "viewer@example.com", Name: "Viewer", Emails: []scimProfileEmail{{Value: "one@example.com", Primary: true}, {Value: "two@example.com", Primary: true}}}); err == nil {
		t.Fatal("conflicting primary SCIM emails were accepted")
	}
	if _, err := scimAttributeSet(strings.Repeat("username,", 16) + "username"); err == nil {
		t.Fatal("oversized SCIM attribute selection was accepted")
	}
}

func TestSCIMEnterpriseExtensionsAcceptTypedAndRejectUnknownFields(t *testing.T) {
	if _, err := parseEnterprise([]byte(`{"employeeNumber":"42","manager":{"value":"owner"}}`)); err != nil {
		t.Fatalf("valid enterprise extension = %v", err)
	}
	if _, err := parseEnterprise([]byte(`{"manager":"owner"}`)); err != nil {
		t.Fatalf("string manager extension = %v", err)
	}
	if _, err := parseSCIMManager([]byte(`{"value":"owner"}`)); err != nil {
		t.Fatalf("valid manager = %v", err)
	}
	if _, err := parseSCIMManager([]byte(`{"unknown":"owner"}`)); err == nil {
		t.Fatal("unknown manager field was accepted")
	}
}
