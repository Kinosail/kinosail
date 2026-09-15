package scim

import (
	"bytes"
	"encoding/json"
	"strings"
)

type scimNoTargetError struct{ detail string }

func (err *scimNoTargetError) Error() string { return err.detail }

type scimPatchPath struct {
	name, subattribute      string
	filterAttribute         string
	filterValue             string
	filterPrimary           bool
	hasFilter, filterIsBool bool
}

func (path scimPatchPath) canonical() string {
	if path.subattribute == "" {
		return path.name
	}
	return path.name + "." + path.subattribute
}

func parseSCIMPatchPath(raw string) (scimPatchPath, error) { //nolint:cyclop,gocognit // ValuePath parsing has independent grammar and bounds checks.
	var err error
	raw, err = canonicalSCIMPatchPath(raw)
	if err != nil {
		return scimPatchPath{}, err
	}
	if raw == "" {
		return scimPatchPath{}, &scimValidationError{detail: "PatchOp path is required"}
	}
	if !strings.HasPrefix(raw, "emails") {
		return scimPatchPath{name: raw}, nil
	}
	remainder := raw[len("emails"):]
	path := scimPatchPath{name: "emails"}
	if remainder == "" {
		return path, nil
	}
	if remainder[0] == '[' {
		close := strings.IndexByte(remainder, ']')
		if close < 0 {
			return scimPatchPath{}, &scimValidationError{detail: "SCIM email value path is invalid"}
		}
		filter, err := parseSCIMEmailFilter(remainder[1:close])
		if err != nil {
			return scimPatchPath{}, err
		}
		path.filterAttribute, path.filterValue, path.filterPrimary, path.filterIsBool, path.hasFilter = filter.attribute, filter.value, filter.primary, filter.isBool, true
		remainder = remainder[close+1:]
	}
	if remainder != "" {
		if remainder[0] != '.' || len(remainder) == 1 {
			return scimPatchPath{}, &scimValidationError{detail: "SCIM email value path is invalid"}
		}
		path.subattribute = remainder[1:]
	}
	if path.subattribute != "" && path.subattribute != "value" && path.subattribute != "type" && path.subattribute != "primary" {
		return scimPatchPath{}, &scimValidationError{detail: "SCIM email subattribute is not supported"}
	}
	return path, nil
}

type scimEmailFilter struct {
	attribute, value string
	primary, isBool  bool
}

func parseSCIMEmailFilter(raw string) (scimEmailFilter, error) { //nolint:cyclop // The bounded email valuePath grammar is explicit by attribute type.
	separator := strings.IndexAny(strings.TrimSpace(raw), " \t\r\n")
	if separator <= 0 {
		return scimEmailFilter{}, &scimValidationError{detail: "SCIM email filter is invalid"}
	}
	raw = strings.TrimSpace(raw)
	attribute := raw[:separator]
	rest := strings.TrimSpace(raw[separator:])
	operatorEnd := strings.IndexAny(rest, " \t\r\n")
	if operatorEnd <= 0 || !strings.EqualFold(rest[:operatorEnd], "eq") {
		return scimEmailFilter{}, &scimValidationError{detail: "SCIM email filter must use equality"}
	}
	value := strings.TrimSpace(rest[operatorEnd:])
	filter := scimEmailFilter{attribute: attribute}
	switch attribute {
	case "type", "value":
		if len(value) < 2 || value[0] != '"' || json.Unmarshal([]byte(value), &filter.value) != nil || len(filter.value) > 256 {
			return scimEmailFilter{}, &scimValidationError{detail: "SCIM email filter value is invalid"}
		}
	case "primary":
		if json.Unmarshal([]byte(value), &filter.primary) != nil || value == "" {
			return scimEmailFilter{}, &scimValidationError{detail: "SCIM email filter primary value is invalid"}
		}
		filter.isBool = true
	default:
		return scimEmailFilter{}, &scimValidationError{detail: "SCIM email filter attribute is not supported"}
	}
	return filter, nil
}

func (path scimPatchPath) matches(email scimProfileEmail) bool {
	if !path.hasFilter {
		return true
	}
	switch path.filterAttribute {
	case "type":
		return strings.EqualFold(email.Type, path.filterValue)
	case "value":
		return strings.EqualFold(email.Value, path.filterValue)
	case "primary":
		return email.Primary == path.filterPrimary
	default:
		return false
	}
}

func applySCIMEmailPath(state *scimProfileInput, path scimPatchPath, operation string, value json.RawMessage) error {
	defer func() { normalizeSCIMPrimary(state.Emails) }()
	indices := scimEmailIndices(state.Emails, path)
	if path.subattribute == "" {
		return applySCIMEmailObjectPath(state, path, operation, value, indices)
	}
	return applySCIMEmailSubattributePath(state, path, operation, value, indices)
}

func applySCIMEmailObjectPath(state *scimProfileInput, path scimPatchPath, operation string, value json.RawMessage, indices []int) error {
	var email scimEmail
	if err := json.Unmarshal(value, &email); err != nil {
		return &scimValidationError{detail: "email value must be an object"}
	}
	converted, err := scimProfileEmails([]scimEmail{email})
	if err != nil {
		return err
	}
	if len(indices) == 0 {
		if strings.EqualFold(operation, "replace") {
			return &scimNoTargetError{detail: "email target was not found"}
		}
		applySCIMEmailFilter(&converted[0], path)
		state.Emails = append(state.Emails, converted...)
		return nil
	}
	for _, index := range indices {
		state.Emails[index] = converted[0]
	}
	return nil
}

func applySCIMEmailSubattributePath(state *scimProfileInput, path scimPatchPath, operation string, value json.RawMessage, indices []int) error {
	if len(indices) == 0 {
		if !strings.EqualFold(operation, "add") {
			return &scimNoTargetError{detail: "email target was not found"}
		}
		email := scimProfileEmail{}
		if err := setSCIMEmailSubattribute(&email, path.subattribute, value); err != nil {
			return err
		}
		applySCIMEmailFilter(&email, path)
		state.Emails = append(state.Emails, email)
		return nil
	}
	for _, index := range indices {
		if err := setSCIMEmailSubattribute(&state.Emails[index], path.subattribute, value); err != nil {
			return err
		}
	}
	return nil
}

func applySCIMEmailFilter(email *scimProfileEmail, path scimPatchPath) {
	switch path.filterAttribute {
	case "type":
		email.Type = path.filterValue
	case "value":
		email.Value = path.filterValue
	case "primary":
		email.Primary = path.filterPrimary
	}
}

func setSCIMEmailSubattribute(email *scimProfileEmail, subattribute string, value json.RawMessage) error { //nolint:cyclop // Each writable email subattribute has strict independent typing.
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return &scimValidationError{detail: "PatchOp values must not be null"}
	}
	switch subattribute {
	case "value":
		var text string
		if json.Unmarshal(value, &text) != nil {
			return &scimValidationError{detail: "email value must be a string"}
		}
		validated, err := scimString(text, 256, true, "email")
		if err != nil {
			return err
		}
		email.Value = validated
	case "type":
		var text string
		if json.Unmarshal(value, &text) != nil {
			return &scimValidationError{detail: "email type must be a string"}
		}
		validated, err := scimString(text, 64, false, "email type")
		if err != nil {
			return err
		}
		email.Type = validated
	case "primary":
		if json.Unmarshal(value, &email.Primary) != nil {
			return &scimValidationError{detail: "email primary must be a boolean"}
		}
	default:
		return &scimValidationError{detail: "SCIM email subattribute is not supported"}
	}
	return nil
}

func scimEmailIndices(emails []scimProfileEmail, path scimPatchPath) []int {
	indices := make([]int, 0, len(emails))
	for index, email := range emails {
		if path.matches(email) {
			indices = append(indices, index)
		}
	}
	return indices
}
