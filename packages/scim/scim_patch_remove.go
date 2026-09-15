package scim

import "encoding/json"

func removeSCIMPatchField(state *scimProfileInput, rawPath string, _ json.RawMessage) error {
	path, err := parseSCIMPatchPath(rawPath)
	if err != nil {
		return err
	}
	if path.hasFilter {
		return removeFilteredSCIMPatchField(state, path)
	}
	return removeUnfilteredSCIMPatchField(state, path.canonical())
}

func removeFilteredSCIMPatchField(state *scimProfileInput, path scimPatchPath) error {
	indices := scimEmailIndices(state.Emails, path)
	if len(indices) == 0 {
		return &scimNoTargetError{detail: "email target was not found"}
	}
	if path.subattribute == "" || path.subattribute == "value" {
		return removeEmailsAt(state, indices)
	}
	for _, index := range indices {
		if path.subattribute == "type" {
			state.Emails[index].Type = ""
		} else {
			state.Emails[index].Primary = false
		}
	}
	return nil
}

func removeUnfilteredSCIMPatchField(state *scimProfileInput, path string) error {
	if removeSCIMCoreField(state, path) || removeSCIMEnterpriseField(state, path) || removeSCIMEmailField(state, path) {
		return nil
	}
	return &scimValidationError{detail: "PatchOp path is not supported"}
}

func removeSCIMCoreField(state *scimProfileInput, path string) bool {
	switch path {
	case "active":
		state.Active = true
	case "displayname":
		state.Name = state.UserName
	case "name":
		state.Formatted, state.GivenName, state.FamilyName = "", "", ""
	case "name.formatted":
		state.Formatted = ""
	case "name.givenname":
		state.GivenName = ""
	case "name.familyname":
		state.FamilyName = ""
	case "externalid":
		state.ExternalID = ""
	default:
		return false
	}
	return true
}

func removeSCIMEnterpriseField(state *scimProfileInput, path string) bool {
	switch path {
	case "enterprise.employeenumber":
		state.Enterprise.EmployeeNumber = ""
	case "enterprise.costcenter":
		state.Enterprise.CostCenter = ""
	case "enterprise.organization":
		state.Enterprise.Organization = ""
	case "enterprise.division":
		state.Enterprise.Division = ""
	case "enterprise.department":
		state.Enterprise.Department = ""
	case "enterprise.manager":
		state.Enterprise.Manager = scimProfileManager{}
	default:
		return false
	}
	return true
}

func removeSCIMEmailField(state *scimProfileInput, path string) bool {
	switch path {
	case "emails", "emails.value":
		state.Emails = nil
	case "emails.type":
		for index := range state.Emails {
			state.Emails[index].Type = ""
		}
	case "emails.primary":
		for index := range state.Emails {
			state.Emails[index].Primary = false
		}
	default:
		return false
	}
	return true
}
