package scim

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSCIMUsernamePreparationAndEnterpriseValidationEdges(t *testing.T) { //nolint:cyclop // Each normalization layer is a separate trust boundary.
	t.Parallel()
	for _, userName := range []string{"a b", "\u200b"} {
		if _, err := NormalizeProfileInput(ProfileInput{UserName: userName, Name: "Viewer"}); err == nil {
			t.Fatalf("invalid prepared username %q accepted", userName)
		}
	}
	if _, err := NormalizeProfileInput(ProfileInput{UserName: strings.Repeat("İ", 128), Name: "Viewer"}); err == nil {
		t.Fatal("username expanded beyond the normalized size limit")
	}
	if _, err := NormalizeProfileInput(ProfileInput{UserName: "viewer@example.com", Name: "Viewer", Enterprise: EnterpriseProfile{Department: "bad\x00value"}}); err == nil {
		t.Fatal("invalid enterprise field accepted")
	}
	if _, err := parseEnterprise(json.RawMessage(`{"manager":{"unknown":true}}`)); err == nil {
		t.Fatal("invalid enterprise manager accepted")
	}
	if _, err := normalizeEnterprise(scimEnterpriseProfile{Manager: scimProfileManager{Ref: "bad\x00reference"}}); err == nil {
		t.Fatal("invalid enterprise manager reference accepted")
	}
	manager, err := parseSCIMManager(json.RawMessage("null"))
	if err != nil || manager != (scimProfileManager{}) {
		t.Fatalf("null enterprise manager = %#v, %v", manager, err)
	}
}

func TestSCIMOptionalAttributeValidationEdges(t *testing.T) { //nolint:cyclop // Cardinality, shape, key, value, and scalar-type bounds fail independently.
	t.Parallel()
	if err := validateSCIMOptionalLists(json.RawMessage(`[]`), json.RawMessage(`[{"value":1}]`)); err == nil {
		t.Fatal("invalid optional list in sequence accepted")
	}
	tooManyFields := make(map[string]json.RawMessage)
	for index := range 17 {
		tooManyFields[string(rune('a'+index))] = json.RawMessage(`true`)
	}
	if err := validateSCIMOptionalItem(tooManyFields); err == nil {
		t.Fatal("optional item with too many fields accepted")
	}
	for name, item := range map[string]map[string]json.RawMessage{
		"long key":        {strings.Repeat("x", 65): json.RawMessage(`true`)},
		"long value":      {"value": json.RawMessage(`"` + strings.Repeat("x", 2048) + `"`)},
		"malformed value": {"value": json.RawMessage(`{`)},
		"compound value":  {"value": json.RawMessage(`{"nested":true}`)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateSCIMOptionalItem(item); err == nil {
				t.Fatal("invalid optional item accepted")
			}
		})
	}
	if _, err := scimProfileEmails(make([]scimEmail, 17)); err == nil {
		t.Fatal("too many SCIM emails accepted")
	}
	if _, err := scimProfileEmails([]scimEmail{{Value: ""}}); err == nil {
		t.Fatal("blank SCIM email accepted")
	}
	if _, err := scimProfileEmails([]scimEmail{{Value: "viewer@example.com", Type: "bad\x00type"}}); err == nil {
		t.Fatal("invalid SCIM email type accepted")
	}
}

func TestSCIMWireUnmarshalRejectsMalformedNestedValues(t *testing.T) { //nolint:cyclop // Nested wire types retain strict object and required-field semantics.
	t.Parallel()
	for name, target := range map[string]any{
		"name":  &scimName{},
		"email": &scimEmail{},
		"user":  &scimUserPayload{},
	} {
		t.Run(name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(`{"unknown":true}`), target); err == nil {
				t.Fatal("unknown nested field accepted")
			}
		})
	}
	for name, raw := range map[string]string{
		"non-object":    `[]`,
		"missing op":    `{"path":"active","value":true}`,
		"null op":       `{"op":null,"path":"active","value":true}`,
		"unknown field": `{"op":"replace","path":"active","value":true,"unknown":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(raw), &scimPatchOperation{}); err == nil {
				t.Fatal("invalid patch operation accepted")
			}
		})
	}
}

func TestDecodeSCIMBodyRequiresBodyAndHandlesReadFailure(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]io.ReadCloser{
		"missing":      nil,
		"read failure": failingSCIMBody{},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
			request.Header.Set("Content-Type", "application/scim+json")
			request.Body = body
			response := httptest.NewRecorder()
			if decodeSCIMBody(response, request, &scimUserPayload{}, scimUserFields...) || response.Code != http.StatusBadRequest {
				t.Fatalf("decode = %v status=%d", response.Body.String(), response.Code)
			}
		})
	}
}

func TestSCIMObjectRejectsNonObjectsMalformedValuesAndTrailingData(t *testing.T) { //nolint:cyclop // Token-level failures must not produce a partial object.
	t.Parallel()
	for name, raw := range map[string]string{
		"empty":           "",
		"array":           `[]`,
		"malformed field": `{"field":`,
		"malformed close": `{"field":true`,
		"wrong close":     `{"field":true]`,
		"trailing":        `{"field":true}{"second":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			if object, err := scimObject([]byte(raw)); err == nil || object != nil {
				t.Fatalf("invalid object = %#v, %v", object, err)
			}
		})
	}
}

func TestPatternsReturnsIndependentRouteList(t *testing.T) {
	t.Parallel()
	patterns := Patterns()
	if len(patterns) == 0 {
		t.Fatal("SCIM route patterns are empty")
	}
	patterns[0] = "changed"
	if Patterns()[0] == "changed" {
		t.Fatal("SCIM route patterns share mutable storage")
	}
}

type failingSCIMBody struct{}

func (failingSCIMBody) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingSCIMBody) Close() error             { return nil }
