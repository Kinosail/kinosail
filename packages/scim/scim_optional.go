package scim

import (
	"bytes"
	"encoding/json"
)

func validateSCIMOptionalLists(values ...json.RawMessage) error {
	for _, raw := range values {
		if err := validateSCIMOptionalList(raw); err != nil {
			return err
		}
	}
	return nil
}

func validateSCIMOptionalList(raw json.RawMessage) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var items []map[string]json.RawMessage
	if len(raw) > 64<<10 || json.Unmarshal(raw, &items) != nil || len(items) > 32 {
		return &scimValidationError{detail: "optional SCIM attribute is invalid"}
	}
	for _, item := range items {
		if err := validateSCIMOptionalItem(item); err != nil {
			return err
		}
	}
	return nil
}

func validateSCIMOptionalItem(item map[string]json.RawMessage) error {
	if len(item) > 16 {
		return &scimValidationError{detail: "optional SCIM attribute is invalid"}
	}
	for key, value := range item {
		var scalar any
		if len(key) > 64 || len(value) > 2048 || json.Unmarshal(value, &scalar) != nil {
			return &scimValidationError{detail: "optional SCIM attribute is invalid"}
		}
		switch scalar.(type) {
		case nil, string, bool:
		default:
			return &scimValidationError{detail: "optional SCIM attribute is invalid"}
		}
	}
	return nil
}

func scimProfileEmails(emails []scimEmail) ([]scimProfileEmail, error) {
	if len(emails) > 16 {
		return nil, &scimValidationError{detail: "emails contains too many values"}
	}
	result := make([]scimProfileEmail, 0, len(emails))
	primary := false
	for _, email := range emails {
		value, err := scimString(email.Value, 256, true, "email")
		if err != nil {
			return nil, err
		}
		typeName, err := scimString(email.Type, 64, false, "email type")
		if err != nil {
			return nil, err
		}
		if email.Primary {
			if primary {
				return nil, &scimValidationError{detail: "emails must contain at most one primary value"}
			}
			primary = true
		}
		result = append(result, scimProfileEmail{Value: value, Type: typeName, Primary: email.Primary})
	}
	return result, nil
}

func removeEmailsAt(state *scimProfileInput, indices []int) error {
	removed := make(map[int]bool, len(indices))
	for _, index := range indices {
		removed[index] = true
	}
	filtered := state.Emails[:0]
	for index, email := range state.Emails {
		if !removed[index] {
			filtered = append(filtered, email)
		}
	}
	state.Emails = filtered
	return nil
}
