package scim

import (
	"encoding/json"
	"testing"
)

func assertSCIMPatchFieldMatrix(t *testing.T, state *scimProfileInput) {
	t.Helper()
	for path, value := range map[string]string{
		"active": `false`, "username": `"new@example.com"`, "displayname": `"New"`, "name.formatted": `"New Name"`, "name.givenname": `"New"`, "name.familyname": `"Viewer"`, "externalid": `"new-id"`,
		"emails.value": `"new@example.com"`, "emails.type": `"work"`, "emails.primary": `false`, "name": `{"formatted":"Whole Name","givenName":"Whole","familyName":"Name"}`,
		"enterprise.employeenumber": `"42"`, "enterprise.costcenter": `"Media"`, "enterprise.organization": `"Kinosail"`, "enterprise.division": `"Playback"`,
		"enterprise.department": `"Engineering"`, "enterprise.manager": `{"value":"owner"}`,
	} {
		if err := applySCIMPatchField(state, path, "replace", json.RawMessage(value)); err != nil {
			t.Fatalf("replace %s: %v", path, err)
		}
	}
	if err := applySCIMPatchField(state, "emails", "replace", json.RawMessage(`[{"value":"one@example.com"},{"value":"two@example.com"}]`)); err != nil {
		t.Fatal(err)
	}
	if err := applySCIMPatchField(state, "emails", "add", json.RawMessage(`[{"value":"three@example.com"}]`)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"active", "displayname", "name.formatted", "name.givenname", "name.familyname", "externalid", "emails.type", "emails.primary", "emails.value", "emails",
		"enterprise.employeenumber", "enterprise.costcenter", "enterprise.organization", "enterprise.division", "enterprise.department", "enterprise.manager",
	} {
		if err := removeSCIMPatchField(state, path, nil); err != nil {
			t.Fatalf("remove %s: %v", path, err)
		}
	}
	if err := applySCIMEnterpriseText(state, "enterprise.department", json.RawMessage(`false`)); err == nil {
		t.Fatal("non-string enterprise patch was accepted")
	}
	if err := applySCIMEnterpriseText(state, "enterprise.department", json.RawMessage(`""`)); err == nil {
		t.Fatal("empty enterprise patch was accepted")
	}
	if removeSCIMEnterpriseField(state, "enterprise.unknown") {
		t.Fatal("unknown enterprise removal was accepted")
	}
}

func assertSCIMNullPatchFields(t *testing.T, state *scimProfileInput) {
	t.Helper()
	if _, err := applySCIMPatch(viewerProfile{UserName: "viewer@example.com"}, []scimPatchOperation{{Op: "remove", Value: json.RawMessage(`{"displayName":true}`)}}); err == nil {
		t.Fatal("SCIM remove without a path was accepted")
	}
	for _, path := range []string{"active", "displayname", "externalid", "emails.value", "emails.type", "emails.primary"} {
		if err := applySCIMPatchField(state, path, "replace", json.RawMessage(`null`)); err != nil {
			t.Fatalf("null %s did not clear the optional value: %v", path, err)
		}
	}
	if err := applySCIMPatchField(state, "username", "replace", json.RawMessage(`null`)); err == nil {
		t.Fatal("required userName accepted null")
	}
	if err := setSCIMEmailSubattribute(&scimProfileEmail{}, "value", json.RawMessage(`1`)); err == nil {
		t.Fatal("non-string email value was accepted")
	}
	if err := setSCIMEmailSubattribute(&scimProfileEmail{}, "primary", json.RawMessage(`"yes"`)); err == nil {
		t.Fatal("non-boolean email primary was accepted")
	}
	var operation scimPatchOperation
	if err := json.Unmarshal([]byte(`{"op":"replace","path":null,"value":true}`), &operation); err == nil {
		t.Fatal("null SCIM patch path was accepted")
	}
	if err := json.Unmarshal([]byte(`{"op":"replace","path":"active","value":null}`), &operation); err != nil || !operation.ValuePresent {
		t.Fatal("null SCIM patch value was not preserved")
	}
}

func assertSCIMPatchPrecedenceAndPrimary(t *testing.T) {
	t.Helper()
	pathless, err := applySCIMPatch(viewerProfile{UserName: "viewer@example.com", Name: "Old", NameParts: scimProfileName{Formatted: "Old"}}, []scimPatchOperation{{Op: "replace", Value: json.RawMessage(`{"displayName":"Display","name":{"formatted":"Name"}}`)}})
	if err != nil || pathless.Name != "Display" {
		t.Fatalf("pathless SCIM patch precedence = %#v err=%v", pathless, err)
	}
	state := scimProfileInput{UserName: "viewer@example.com", Name: "Old", Formatted: "Old", Active: true}
	if err := applySCIMPatchField(&state, "name.formatted", "replace", json.RawMessage(`"New Name"`)); err != nil || state.Name != "Old" {
		t.Fatalf("name subattribute changed displayName = %#v err=%v", state, err)
	}
	state = scimProfileInput{UserName: "viewer@example.com", Name: "Viewer", Active: true, Emails: []scimProfileEmail{{Value: "one@example.com", Primary: true}, {Value: "two@example.com"}}}
	if err := applySCIMPatchField(&state, `emails[value eq "two@example.com"].primary`, "replace", json.RawMessage(`true`)); err != nil {
		t.Fatal(err)
	}
	primary := 0
	for _, email := range state.Emails {
		if email.Primary {
			primary++
		}
	}
	if primary != 1 || !state.Emails[1].Primary {
		t.Fatalf("SCIM email primary state = %#v", state.Emails)
	}
}
