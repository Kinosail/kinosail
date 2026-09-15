package scim

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func scimResource(profile viewerProfile) scimUserResource {
	location := "/scim/v2/Users/" + url.PathEscape(profile.ID)
	resource := scimUserResource{Schemas: []string{scimUserSchema}, ID: profile.ID, ExternalID: profile.ExternalID, UserName: profile.UserName, Name: scimResourceName{Formatted: profile.NameParts.Formatted, GivenName: profile.NameParts.GivenName, FamilyName: profile.NameParts.FamilyName}, DisplayName: profile.Name, Active: !profile.Disabled, Meta: scimMeta{ResourceType: "User", Created: profile.CreatedAt.Format(time.RFC3339Nano), LastModified: profile.UpdatedAt.Format(time.RFC3339Nano), Location: location, Version: `W/"` + strconv.FormatUint(profile.Revision, 10) + `"`}}
	for _, email := range profile.Emails {
		resource.Emails = append(resource.Emails, scimEmail{Value: email.Value, Type: email.Type, Primary: email.Primary}) //nolint:staticcheck // Wire and persisted structs intentionally use different tags.
	}
	if profile.Enterprise != (scimEnterpriseProfile{}) {
		resource.Schemas = append(resource.Schemas, scimEnterpriseUser)
	}
	return resource
}

func validateSCIMSchema(schemas []string, wanted string) error {
	if len(schemas) != 1 || schemas[0] != wanted {
		return &scimValidationError{detail: "the request must contain exactly one supported SCIM schema"}
	}
	return nil
}

func validateSCIMUserSchemas(schemas []string) error {
	if len(schemas) == 1 && schemas[0] == scimUserSchema || len(schemas) == 2 && (schemas[0] == scimUserSchema && schemas[1] == scimEnterpriseUser || schemas[1] == scimUserSchema && schemas[0] == scimEnterpriseUser) {
		return nil
	}
	return &scimValidationError{detail: "the request must contain the core user schema and only supported SCIM schemas"}
}

func scimResourceValue(profile viewerProfile, selection scimSelection) map[string]any {
	resource := scimResource(profile)
	value := map[string]any{"schemas": resource.Schemas, "id": resource.ID}
	addSCIMCoreValue(value, resource, selection)
	if selection.includes("emails") && len(resource.Emails) != 0 {
		value["emails"] = scimEmailValues(resource.Emails, selection)
	}
	if selection.includes("meta") {
		value["meta"] = resource.Meta
	}
	if enterprise, ok := scimEnterpriseValue(profile, selection); ok {
		value[scimEnterpriseUser] = enterprise
	}
	return value
}

func addSCIMCoreValue(value map[string]any, resource scimUserResource, selection scimSelection) {
	if selection.includes("externalid") && resource.ExternalID != "" {
		value["externalId"] = resource.ExternalID
	}
	if selection.includes("username") {
		value["userName"] = resource.UserName
	}
	if selection.includes("name") {
		value["name"] = scimNameValue(resource.Name, selection)
	}
	if selection.includes("displayname") {
		value["displayName"] = resource.DisplayName
	}
	if selection.includes("active") {
		value["active"] = resource.Active
	}
}

func scimNameValue(name scimResourceName, selection scimSelection) map[string]string {
	value := make(map[string]string)
	if selection.includesSub("name", "formatted") {
		value["formatted"] = name.Formatted
	}
	if selection.includesSub("name", "givenname") {
		value["givenName"] = name.GivenName
	}
	if selection.includesSub("name", "familyname") {
		value["familyName"] = name.FamilyName
	}
	return value
}

func scimEmailValues(emails []scimEmail, selection scimSelection) []map[string]any {
	values := make([]map[string]any, 0, len(emails))
	for _, email := range emails {
		item := map[string]any{}
		if selection.includesSub("emails", "value") {
			item["value"] = email.Value
		}
		if selection.includesSub("emails", "type") && email.Type != "" {
			item["type"] = email.Type
		}
		if selection.includesSub("emails", "primary") && email.Primary {
			item["primary"] = true
		}
		values = append(values, item)
	}
	return values
}

func scimEnterpriseValue(profile viewerProfile, selection scimSelection) (any, bool) {
	if !selection.includes("enterprise") || profile.Enterprise == (scimEnterpriseProfile{}) {
		return nil, false
	}
	if len(selection.included) == 0 && len(selection.excluded) == 0 {
		return profile.Enterprise, true
	}
	profileValue := profile.Enterprise
	value := make(map[string]any)
	fields := []struct {
		path, name, value string
	}{
		{"employeenumber", "employeeNumber", profileValue.EmployeeNumber},
		{"costcenter", "costCenter", profileValue.CostCenter},
		{"organization", "organization", profileValue.Organization},
		{"division", "division", profileValue.Division},
		{"department", "department", profileValue.Department},
	}
	for _, field := range fields {
		if selection.includesSub("enterprise", field.path) && field.value != "" {
			value[field.name] = field.value
		}
	}
	if manager, ok := scimManagerValue(profileValue.Manager, selection); ok {
		value["manager"] = manager
	}
	return value, true
}

func scimManagerValue(profile scimProfileManager, selection scimSelection) (map[string]string, bool) {
	if !selection.includes("enterprise.manager") || profile == (scimProfileManager{}) {
		return nil, false
	}
	value := make(map[string]string)
	fields := []struct {
		path, name, value string
	}{
		{"value", "value", profile.Value},
		{"$ref", "$ref", profile.Ref},
		{"displayname", "displayName", profile.DisplayName},
	}
	for _, field := range fields {
		if selection.includesSub("enterprise.manager", field.path) && field.value != "" {
			value[field.name] = field.value
		}
	}
	return value, true
}

func (selection scimSelection) includes(attribute string) bool {
	attribute = strings.ToLower(attribute)
	if attribute == "id" || attribute == "schemas" {
		return true
	}
	if selection.excluded[attribute] {
		return false
	}
	if selection.included[attribute] {
		return true
	}
	for path := range selection.included {
		if strings.HasPrefix(path, attribute+".") {
			return true
		}
	}
	return len(selection.included) == 0
}

func (selection scimSelection) includesSub(attribute, subattribute string) bool {
	attribute, subattribute = strings.ToLower(attribute), strings.ToLower(subattribute)
	if selection.excluded[attribute] || selection.excluded[attribute+"."+subattribute] {
		return false
	}
	return len(selection.included) == 0 || selection.included[attribute] || selection.included[attribute+"."+subattribute]
}

func scimResourceHeaders(writer http.ResponseWriter, profile viewerProfile, includeLocation bool) {
	resource := scimResource(profile)
	writer.Header().Set("ETag", resource.Meta.Version)
	writer.Header().Set("Content-Location", resource.Meta.Location)
	if includeLocation {
		writer.Header().Set("Location", resource.Meta.Location)
	}
}

func scimConditional(writer http.ResponseWriter, request *http.Request, profile viewerProfile, mutate bool) bool { //nolint:cyclop,gocognit // Conditional HTTP exits stay in protocol order.
	resource := scimResource(profile)
	ifMatch, ifNoneMatch := strings.TrimSpace(request.Header.Get("If-Match")), strings.TrimSpace(request.Header.Get("If-None-Match"))
	if len(request.Header.Values("If-Match")) > 1 || len(request.Header.Values("If-None-Match")) > 1 || ifMatch != "" && ifNoneMatch != "" || !validSCIMETagHeader(ifMatch) || !validSCIMETagHeader(ifNoneMatch) {
		writeSCIMError(writer, http.StatusBadRequest, "invalidValue", "conditional headers are invalid")
		return true
	}
	if mutate {
		if ifNoneMatch != "" && ETagMatches(ifNoneMatch, resource.Meta.Version) {
			writeSCIMError(writer, http.StatusPreconditionFailed, "invalidValue", "If-None-Match precondition failed")
			return true
		}
		return false
	}
	if ETagMatches(ifNoneMatch, resource.Meta.Version) {
		scimResourceHeaders(writer, profile, false)
		writer.WriteHeader(http.StatusNotModified)
		return true
	}
	return false
}

func validSCIMETagHeader(header string) bool { //nolint:cyclop,gocognit // Entity-tag grammar is deliberately explicit.
	if header == "" {
		return true
	}
	values := strings.Split(header, ",")
	if len(header) > 1024 || len(values) > 16 {
		return false
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "*" {
			continue
		}
		value = strings.TrimPrefix(value, "W/")
		if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
			return false
		}
		for _, character := range []byte(value[1 : len(value)-1]) {
			if character != 0x21 && (character < 0x23 || character == 0x7f) {
				return false
			}
		}
	}
	return true
}

// ETagMatches compares a validated conditional header with one current entity tag.
func ETagMatches(header, current string) bool {
	if header == "*" {
		return true
	}
	for _, candidate := range strings.Split(header, ",") {
		if strings.TrimSpace(candidate) == current {
			return true
		}
	}
	return false
}

func writeSCIMResource(writer http.ResponseWriter, profile viewerProfile, selection scimSelection, status int, includeLocation bool) {
	scimResourceHeaders(writer, profile, includeLocation)
	writeSCIMJSON(writer, scimResourceValue(profile, selection), status)
}
