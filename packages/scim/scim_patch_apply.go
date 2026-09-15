package scim

import (
	"bytes"
	"encoding/json"
	"strings"
)

func applySCIMPatchOperation(state *scimProfileInput, operation scimPatchOperation) error {
	hasPath := operation.PathPresent || strings.TrimSpace(operation.Path) != ""
	if operation.PathPresent && strings.TrimSpace(operation.Path) == "" {
		return &scimNoTargetError{detail: "PatchOp path must not be blank"}
	}
	if !hasPath && strings.EqualFold(operation.Op, "remove") {
		return &scimNoTargetError{detail: "remove requires a target path"}
	}
	if !hasPath {
		return applySCIMObjectPatch(state, operation)
	}
	path := strings.TrimSpace(operation.Path)
	if strings.EqualFold(operation.Op, "remove") {
		return removeSCIMPatchField(state, path, operation.Value)
	}
	if len(operation.Value) == 0 {
		return &scimValidationError{detail: "PatchOp value is required"}
	}
	return applySCIMPatchField(state, path, operation.Op, operation.Value)
}

func applySCIMObjectPatch(state *scimProfileInput, operation scimPatchOperation) error {
	var fields map[string]json.RawMessage
	if err := unmarshalSCIMObject(operation.Value, &fields, "active", "userName", "displayName", "externalId", "name", "emails"); err != nil {
		return err
	}
	canonicalFields := make(map[string]json.RawMessage, len(fields))
	for field, value := range fields {
		canonicalFields[strings.ToLower(field)] = value
	}
	for _, field := range scimPatchFieldOrder {
		value, present := canonicalFields[field]
		if present {
			if err := applySCIMPatchField(state, field, operation.Op, value); err != nil {
				return err
			}
		}
	}
	if len(canonicalFields) == 0 {
		return &scimValidationError{detail: "PatchOp value must contain a supported attribute"}
	}
	return nil
}

func applySCIMPatchField(state *scimProfileInput, path, operation string, value json.RawMessage) error {
	defer func() { normalizeSCIMPrimary(state.Emails) }()
	if operation != "add" && operation != "replace" && !strings.EqualFold(operation, "add") && !strings.EqualFold(operation, "replace") {
		return &scimValidationError{detail: "PatchOp operation is not supported"}
	}
	parsedPath, err := parseSCIMPatchPath(path)
	if err != nil {
		return err
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return removeSCIMPatchField(state, path, nil)
	}
	if parsedPath.hasFilter {
		return applySCIMEmailPath(state, parsedPath, operation, value)
	}
	canonical := parsedPath.canonical()
	handlers := []func(*scimProfileInput, string, string, json.RawMessage) (bool, error){applySCIMCorePatchField, applySCIMEnterprisePatchField, applySCIMEmailPatchField, applySCIMNamePatchField}
	for _, handler := range handlers {
		if handled, err := handler(state, canonical, operation, value); handled {
			return err
		}
	}
	return &scimValidationError{detail: "PatchOp path is not supported"}
}

func applySCIMCorePatchField(state *scimProfileInput, path, _ string, value json.RawMessage) (bool, error) {
	var text string
	switch path {
	case "active":
		if json.Unmarshal(value, &state.Active) != nil {
			return true, &scimValidationError{detail: "active must be a boolean"}
		}
	case "username":
		if json.Unmarshal(value, &text) != nil {
			return true, &scimValidationError{detail: "userName must be a string"}
		}
		state.UserName = text
	case "externalid":
		if json.Unmarshal(value, &text) != nil {
			return true, &scimValidationError{detail: "externalId must be a string"}
		}
		state.ExternalID = text
	default:
		return applySCIMDisplayPatchField(state, path, value)
	}
	return true, nil
}

func applySCIMDisplayPatchField(state *scimProfileInput, path string, value json.RawMessage) (bool, error) {
	if path != "displayname" && path != "name.formatted" && path != "name.givenname" && path != "name.familyname" {
		return false, nil
	}
	var text string
	if json.Unmarshal(value, &text) != nil {
		return true, &scimValidationError{detail: "display name must be a string"}
	}
	switch path {
	case "displayname":
		state.Name = text
	case "name.formatted":
		state.Formatted = text
	case "name.givenname":
		state.GivenName = text
	case "name.familyname":
		state.FamilyName = text
	}
	return true, nil
}

func applySCIMEnterprisePatchField(state *scimProfileInput, path, _ string, value json.RawMessage) (bool, error) {
	switch path {
	case "enterprise.employeenumber", "enterprise.costcenter", "enterprise.organization", "enterprise.division", "enterprise.department":
		return true, applySCIMEnterpriseText(state, path, value)
	case "enterprise.manager":
		manager, err := parseSCIMManager(value)
		if err != nil {
			return true, err
		}
		state.Enterprise.Manager = manager
	default:
		return false, nil
	}
	return true, nil
}

func applySCIMEnterpriseText(state *scimProfileInput, path string, value json.RawMessage) error {
	var text string
	if json.Unmarshal(value, &text) != nil {
		return &scimValidationError{detail: "enterprise attribute must be a string"}
	}
	text, err := scimString(text, 2048, true, "enterprise attribute")
	if err != nil {
		return err
	}
	switch path {
	case "enterprise.employeenumber":
		state.Enterprise.EmployeeNumber = text
	case "enterprise.costcenter":
		state.Enterprise.CostCenter = text
	case "enterprise.organization":
		state.Enterprise.Organization = text
	case "enterprise.division":
		state.Enterprise.Division = text
	case "enterprise.department":
		state.Enterprise.Department = text
	}
	return nil
}

func applySCIMEmailPatchField(state *scimProfileInput, path, operation string, value json.RawMessage) (bool, error) {
	switch path {
	case "emails":
		return true, applySCIMEmails(state, operation, value)
	case "emails.value":
		return true, applySCIMEmailValue(state, operation, value)
	case "emails.type":
		return true, applySCIMEmailType(state, value)
	case "emails.primary":
		return true, applySCIMEmailPrimary(state, value)
	default:
		return false, nil
	}
}

func applySCIMEmails(state *scimProfileInput, operation string, value json.RawMessage) error {
	var emails []scimEmail
	if json.Unmarshal(value, &emails) != nil {
		return &scimValidationError{detail: "emails must be an array"}
	}
	converted, err := scimProfileEmails(emails)
	if err != nil {
		return err
	}
	if strings.EqualFold(operation, "add") {
		state.Emails = append(state.Emails, converted...)
	} else {
		state.Emails = converted
	}
	return nil
}

func applySCIMEmailValue(state *scimProfileInput, operation string, value json.RawMessage) error {
	var text string
	if json.Unmarshal(value, &text) != nil {
		return &scimValidationError{detail: "email value must be a string"}
	}
	emailValue, err := scimString(text, 256, true, "email")
	if err != nil {
		return err
	}
	if strings.EqualFold(operation, "add") || len(state.Emails) == 0 {
		state.Emails = append(state.Emails, scimProfileEmail{Value: emailValue})
	} else {
		state.Emails[0].Value = emailValue
	}
	return nil
}

func applySCIMEmailType(state *scimProfileInput, value json.RawMessage) error {
	var text string
	if json.Unmarshal(value, &text) != nil {
		return &scimValidationError{detail: "email type must be a string"}
	}
	text, err := scimString(text, 64, false, "email type")
	if err != nil {
		return err
	}
	if len(state.Emails) == 0 {
		state.Emails = append(state.Emails, scimProfileEmail{Type: text})
	} else {
		state.Emails[0].Type = text
	}
	return nil
}

func applySCIMEmailPrimary(state *scimProfileInput, value json.RawMessage) error {
	var primary bool
	if json.Unmarshal(value, &primary) != nil {
		return &scimValidationError{detail: "email primary must be a boolean"}
	}
	if len(state.Emails) == 0 {
		return &scimNoTargetError{detail: "email target was not found"}
	}
	state.Emails[0].Primary = primary
	return nil
}

func applySCIMNamePatchField(state *scimProfileInput, path, operation string, value json.RawMessage) (bool, error) {
	if path != "name" {
		return false, nil
	}
	var name scimName
	if err := json.Unmarshal(value, &name); err != nil {
		return true, err
	}
	if strings.EqualFold(operation, "add") {
		if name.Formatted != "" {
			state.Formatted = name.Formatted
		}
		if name.GivenName != "" {
			state.GivenName = name.GivenName
		}
		if name.FamilyName != "" {
			state.FamilyName = name.FamilyName
		}
	} else {
		state.Formatted, state.GivenName, state.FamilyName = name.Formatted, name.GivenName, name.FamilyName
	}
	return true, nil
}

func normalizeSCIMPrimary(emails []scimProfileEmail) {
	primary := -1
	for index := range emails {
		if emails[index].Primary {
			primary = index
		}
	}
	if primary < 0 {
		return
	}
	for index := range emails {
		emails[index].Primary = index == primary
	}
}
