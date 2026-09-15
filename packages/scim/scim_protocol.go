package scim

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

type scimName struct {
	Formatted  string `json:"formatted"`
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
}

func (name *scimName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*name = scimName{}
		return nil
	}
	var value struct {
		Formatted  string `json:"formatted"`
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	}
	if err := unmarshalSCIMObject(data, &value, "formatted", "givenName", "familyName", "middleName", "honorificPrefix", "honorificSuffix"); err != nil {
		return err
	}
	*name = scimName(value)
	return nil
}

type scimEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

func (email *scimEmail) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("email must be an object")
	}
	var value struct {
		Value   string `json:"value"`
		Type    string `json:"type"`
		Primary bool   `json:"primary"`
	}
	if err := unmarshalSCIMObject(data, &value, "value", "type", "primary"); err != nil {
		return err
	}
	*email = scimEmail(value)
	return nil
}

type scimUserPayload struct {
	Schemas      []string          `json:"schemas"`
	ID           string            `json:"id"`
	UserName     string            `json:"userName"`
	Name         scimName          `json:"name"`
	DisplayName  string            `json:"displayName"`
	Active       *bool             `json:"active"`
	ExternalID   string            `json:"externalId"`
	Emails       []scimEmail       `json:"emails"`
	Roles        []json.RawMessage `json:"roles"`
	Password     string            `json:"password"`
	Meta         json.RawMessage   `json:"meta"` // The RFC-defined read-only metadata is accepted and ignored.
	Enterprise   json.RawMessage   `json:"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"`
	Title        string            `json:"title"`
	UserType     string            `json:"userType"`
	NickName     string            `json:"nickName"`
	ProfileURL   string            `json:"profileUrl"`
	Language     string            `json:"preferredLanguage"`
	Locale       string            `json:"locale"`
	Timezone     string            `json:"timezone"`
	PhoneNumbers json.RawMessage   `json:"phoneNumbers"`
	Addresses    json.RawMessage   `json:"addresses"`
	Photos       json.RawMessage   `json:"photos"`
	IMs          json.RawMessage   `json:"ims"`
	Entitlements json.RawMessage   `json:"entitlements"`
	Certificates json.RawMessage   `json:"x509Certificates"`
	Groups       json.RawMessage   `json:"groups"`
}

var scimUserFields = []string{"schemas", "id", "userName", "name", "displayName", "active", "externalId", "emails", "roles", "password", "meta", scimEnterpriseUser, "title", "userType", "nickName", "profileUrl", "preferredLanguage", "locale", "timezone", "phoneNumbers", "addresses", "photos", "ims", "entitlements", "x509Certificates", "groups"}

func (payload *scimUserPayload) UnmarshalJSON(data []byte) error {
	type plainSCIMUserPayload scimUserPayload
	var value plainSCIMUserPayload
	if err := unmarshalSCIMObject(data, &value, scimUserFields...); err != nil {
		return err
	}
	*payload = scimUserPayload(value)
	return nil
}

type scimPatchOperation struct {
	Op           string          `json:"op"`
	Path         string          `json:"path"`
	Value        json.RawMessage `json:"value"`
	PathPresent  bool            `json:"-"`
	ValuePresent bool            `json:"-"`
}

func (operation *scimPatchOperation) UnmarshalJSON(data []byte) error {
	object, err := scimObject(data)
	if err != nil {
		return err
	}
	if raw, present := object["op"]; !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("PatchOp op must be present and non-null")
	}
	pathPresent := false
	if raw, present := object["path"]; present {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return errors.New("PatchOp path must not be null")
		}
		pathPresent = true
	}
	valuePresent := false
	if _, present := object["value"]; present {
		valuePresent = true
	}
	var value struct {
		Op    string          `json:"op"`
		Path  string          `json:"path"`
		Value json.RawMessage `json:"value"`
	}
	if err := unmarshalSCIMObject(data, &value, "op", "path", "value"); err != nil {
		return err
	}
	*operation = scimPatchOperation{Op: value.Op, Path: value.Path, Value: value.Value, PathPresent: pathPresent, ValuePresent: valuePresent}
	return nil
}

type scimPatchPayload struct {
	Schemas    []string             `json:"schemas"`
	Operations []scimPatchOperation `json:"Operations"`
}

type scimUserResource struct {
	Schemas     []string         `json:"schemas"`
	ID          string           `json:"id"`
	ExternalID  string           `json:"externalId,omitempty"`
	UserName    string           `json:"userName"`
	Name        scimResourceName `json:"name"`
	DisplayName string           `json:"displayName"`
	Active      bool             `json:"active"`
	Emails      []scimEmail      `json:"emails,omitempty"`
	Meta        scimMeta         `json:"meta"`
}

type scimResourceName struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type scimMeta struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created"`
	LastModified string `json:"lastModified"`
	Location     string `json:"location"`
	Version      string `json:"version"`
}

func decodeSCIMBody(writer http.ResponseWriter, request *http.Request, target any, fields ...string) bool {
	contentTypes := request.Header.Values("Content-Type")
	mediaType, _, mediaErr := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if len(contentTypes) != 1 || mediaErr != nil || mediaType != "application/scim+json" && mediaType != "application/json" {
		writeSCIMError(writer, http.StatusUnsupportedMediaType, "invalidSyntax", "Content-Type must be application/scim+json or application/json")
		return false
	}
	if request.Body == nil {
		writeSCIMError(writer, http.StatusBadRequest, "invalidSyntax", "the SCIM request body is required")
		return false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxSCIMBody)
	data, err := io.ReadAll(request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeSCIMError(writer, http.StatusRequestEntityTooLarge, "", "the SCIM request body exceeds the 1048576 byte limit")
			return false
		}
	}
	if err == nil {
		err = unmarshalSCIMObject(data, target, fields...)
	}
	if err != nil {
		writeSCIMError(writer, http.StatusBadRequest, "invalidSyntax", "the SCIM request body is invalid")
		return false
	}
	return true
}

func unmarshalSCIMObject(data []byte, target any, fields ...string) error {
	object, err := scimObject(data)
	if err != nil {
		return err
	}
	allowed := make(map[string]bool, len(fields))
	for _, field := range fields {
		allowed[strings.ToLower(field)] = true
	}
	for field := range object {
		if !allowed[strings.ToLower(field)] {
			return errors.New("SCIM field is not supported")
		}
	}
	return json.Unmarshal(data, target)
}

func scimObject(data []byte) (map[string]json.RawMessage, error) { //nolint:cyclop // Token-level parsing rejects malformed and ambiguous objects at one boundary.
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	delimiter, isDelimiter := token.(json.Delim)
	if err != nil || !isDelimiter || delimiter != '{' {
		return nil, errors.New("SCIM body must be a JSON object")
	}
	object := make(map[string]json.RawMessage)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, errors.New("SCIM body must be valid JSON")
		}
		field := key.(string)
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, errors.New("SCIM body must be valid JSON")
		}
		canonical := strings.ToLower(field)
		if _, found := object[canonical]; found {
			return nil, errors.New("SCIM body contains duplicate fields")
		}
		object[canonical] = value
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, errors.New("SCIM body must be valid JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("SCIM body must contain one JSON object")
	}
	return object, nil
}
