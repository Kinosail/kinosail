package scim

import (
	"encoding/json"
	"strings"
)

var scimPatchFieldOrder = []string{"active", "username", "displayname", "externalid", "name", "emails"}

func (payload scimUserPayload) profile(requireSchema bool) (scimProfileInput, error) { //nolint:gocognit,cyclop // SCIM field validation and profile projection stay together.
	if requireSchema {
		if err := validateSCIMUserSchemas(payload.Schemas); err != nil {
			return scimProfileInput{}, err
		}
	}
	roles, err := json.Marshal(payload.Roles)
	if err != nil || validateSCIMOptionalLists(roles) != nil {
		return scimProfileInput{}, &scimValidationError{detail: "roles is invalid"}
	}
	enterprise, err := parseEnterprise(payload.Enterprise)
	if err != nil {
		return scimProfileInput{}, err
	}
	for field, value := range map[string]string{"title": payload.Title, "userType": payload.UserType, "nickName": payload.NickName, "profileUrl": payload.ProfileURL, "preferredLanguage": payload.Language, "locale": payload.Locale, "timezone": payload.Timezone} {
		if _, err := scimString(value, 2048, false, field); err != nil {
			return scimProfileInput{}, err
		}
	}
	if err := validateSCIMOptionalLists(payload.PhoneNumbers, payload.Addresses, payload.Photos, payload.IMs, payload.Entitlements, payload.Certificates, payload.Groups); err != nil {
		return scimProfileInput{}, err
	}
	for field, value := range map[string]string{"formatted name": payload.Name.Formatted, "given name": payload.Name.GivenName, "family name": payload.Name.FamilyName} {
		if _, err := scimString(value, 256, false, field); err != nil {
			return scimProfileInput{}, err
		}
	}
	emails, err := scimProfileEmails(payload.Emails)
	if err != nil {
		return scimProfileInput{}, err
	}
	name := strings.TrimSpace(payload.DisplayName)
	if name == "" {
		name = strings.TrimSpace(payload.Name.Formatted)
	}
	if name == "" {
		name = strings.TrimSpace(strings.Join([]string{payload.Name.GivenName, payload.Name.FamilyName}, " "))
	}
	if name == "" {
		name = strings.TrimSpace(payload.UserName)
	}
	active := true
	if payload.Active != nil {
		active = *payload.Active
	}
	return NormalizeProfileInput(scimProfileInput{UserName: payload.UserName, Name: name, Formatted: payload.Name.Formatted, GivenName: payload.Name.GivenName, FamilyName: payload.Name.FamilyName, ExternalID: payload.ExternalID, Emails: emails, Enterprise: enterprise, Active: active})
}

func applySCIMPatch(profile viewerProfile, operations []scimPatchOperation) (scimProfileInput, error) {
	state := scimProfileInput{UserName: profile.UserName, Name: profile.Name, Formatted: profile.NameParts.Formatted, GivenName: profile.NameParts.GivenName, FamilyName: profile.NameParts.FamilyName, ExternalID: profile.ExternalID, Emails: append([]scimProfileEmail(nil), profile.Emails...), Enterprise: profile.Enterprise, Active: !profile.Disabled}
	for _, operation := range operations {
		if err := applySCIMPatchOperation(&state, operation); err != nil {
			return scimProfileInput{}, err
		}
	}
	return NormalizeProfileInput(state)
}
