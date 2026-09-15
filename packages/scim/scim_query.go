package scim

import (
	"encoding/json"
	"strings"
)

func scimFilter(raw string) (func(viewerProfile) bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts, err := splitSCIMAnd(raw)
	if err != nil {
		return nil, err
	}
	if len(parts) > 1 {
		return scimAndFilter(parts)
	}
	return scimSingleFilter(raw)
}

func scimAndFilter(parts []string) (func(viewerProfile) bool, error) {
	filters := make([]func(viewerProfile) bool, 0, len(parts))
	for _, part := range parts {
		filter, err := scimFilter(part)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	return func(profile viewerProfile) bool {
		for _, filter := range filters {
			if !filter(profile) {
				return false
			}
		}
		return true
	}, nil
}

func scimSingleFilter(raw string) (func(viewerProfile) bool, error) {
	separator := scimFilterAttributeEnd(raw)
	if separator <= 0 {
		return nil, &scimValidationError{detail: "SCIM filter syntax is invalid"}
	}
	attribute, emailPath, err := scimFilterPath(raw[:separator])
	if err != nil {
		return nil, err
	}
	rest := strings.TrimSpace(raw[separator:])
	if strings.EqualFold(rest, "pr") {
		return func(profile viewerProfile) bool { return len(scimFilterValues(profile, attribute, emailPath)) != 0 }, nil
	}
	operator, value, err := parseSCIMFilterComparison(attribute, rest)
	if err != nil {
		return nil, err
	}
	return scimComparisonFilter(attribute, emailPath, operator, value), nil
}

func scimFilterPath(raw string) (string, scimPatchPath, error) {
	attribute, err := canonicalSCIMPath(raw)
	if err != nil {
		return "", scimPatchPath{}, err
	}
	allowed := map[string]bool{"id": true, "username": true, "externalid": true, "displayname": true, "name.formatted": true, "name.givenname": true, "name.familyname": true, "active": true, "emails.value": true}
	var emailPath scimPatchPath
	if strings.HasPrefix(attribute, "emails[") {
		emailPath, err = parseSCIMPatchPath(raw)
		allowed[attribute] = err == nil && emailPath.name == "emails" && emailPath.subattribute == "value"
	}
	if !allowed[attribute] {
		return "", scimPatchPath{}, &scimValidationError{detail: "SCIM filter attribute is not supported"}
	}
	return attribute, emailPath, nil
}

func parseSCIMFilterComparison(attribute, rest string) (string, string, error) {
	operatorEnd := strings.IndexAny(rest, " \t\r\n")
	if operatorEnd <= 0 {
		return "", "", &scimValidationError{detail: "SCIM filter operator is invalid"}
	}
	operator := strings.ToLower(rest[:operatorEnd])
	rest = strings.TrimSpace(rest[operatorEnd:])
	if operator != "eq" && operator != "sw" && operator != "co" {
		return "", "", &scimValidationError{detail: "SCIM filter operator is not supported"}
	}
	if attribute == "active" {
		return parseSCIMActiveComparison(operator, rest)
	}
	value, err := parseSCIMTextComparison(rest)
	if err != nil {
		return "", "", err
	}
	if attribute == "username" {
		value, err = prepareUserName(value)
		if err != nil {
			return "", "", err
		}
	}
	return operator, value, nil
}

func parseSCIMActiveComparison(operator, value string) (string, string, error) {
	if operator != "eq" || value != "true" && value != "false" {
		return "", "", &scimValidationError{detail: "SCIM active filter is invalid"}
	}
	return operator, value, nil
}

func parseSCIMTextComparison(raw string) (string, error) {
	var value string
	if len(raw) < 2 || raw[0] != '"' || json.Unmarshal([]byte(raw), &value) != nil || len(value) > 256 {
		return "", &scimValidationError{detail: "SCIM filter value is invalid"}
	}
	return value, nil
}

func scimComparisonFilter(attribute string, emailPath scimPatchPath, operator, value string) func(viewerProfile) bool {
	if attribute == "active" {
		want := value == "true"
		return func(profile viewerProfile) bool { return !profile.Disabled == want }
	}
	return func(profile viewerProfile) bool {
		for _, actual := range scimFilterValues(profile, attribute, emailPath) {
			matches := scimFilterMatch(actual, value, operator, attribute == "externalid" || attribute == "id")
			if attribute == "username" && operator == "eq" {
				matches = SameUserName(actual, value)
			}
			if matches {
				return true
			}
		}
		return false
	}
}

func splitSCIMAnd(raw string) ([]string, error) { //nolint:cyclop,gocognit // Filter splitting tracks quote and bracket state explicitly.
	parts, start, depth, quoted := make([]string, 0, 4), 0, 0, false
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '"':
			if index == 0 || raw[index-1] != '\\' {
				quoted = !quoted
			}
		case '[':
			if !quoted {
				depth++
			}
		case ']':
			if !quoted {
				depth--
			}
		}
		if !quoted && depth == 0 && index+5 <= len(raw) && strings.EqualFold(raw[index:index+5], " and ") {
			parts = append(parts, strings.TrimSpace(raw[start:index]))
			start, index = index+5, index+4
		}
	}
	parts = append(parts, strings.TrimSpace(raw[start:]))
	if quoted || depth != 0 || len(parts) > 4 {
		return nil, &scimValidationError{detail: "SCIM filter syntax is invalid"}
	}
	for _, part := range parts {
		if part == "" {
			return nil, &scimValidationError{detail: "SCIM filter syntax is invalid"}
		}
	}
	return parts, nil
}

func scimFilterAttributeEnd(raw string) int { //nolint:cyclop,gocognit // Attribute parsing tracks quote and bracket state explicitly.
	depth, quoted := 0, false
	for index := range len(raw) {
		switch raw[index] {
		case '"':
			if index == 0 || raw[index-1] != '\\' {
				quoted = !quoted
			}
		case '[':
			if !quoted {
				depth++
			}
		case ']':
			if !quoted {
				depth--
			}
		case ' ', '\t', '\r', '\n':
			if !quoted && depth == 0 {
				return index
			}
		}
	}
	return -1
}

func scimFilterValues(profile viewerProfile, attribute string, emailPath scimPatchPath) []string {
	if attribute == "emails.value" || strings.HasPrefix(attribute, "emails[") {
		values := make([]string, 0, len(profile.Emails))
		for _, email := range profile.Emails {
			if attribute == "emails.value" || emailPath.matches(email) {
				values = append(values, email.Value)
			}
		}
		return values
	}
	value := scimFilterAttribute(profile, attribute)
	if value == "" {
		return nil
	}
	return []string{value}
}

func scimFilterAttribute(profile viewerProfile, attribute string) string {
	switch attribute {
	case "id":
		return profile.ID
	case "username":
		return profile.UserName
	case "externalid":
		return profile.ExternalID
	case "displayname":
		return profile.Name
	case "name.formatted":
		return profile.NameParts.Formatted
	case "name.givenname":
		return profile.NameParts.GivenName
	case "name.familyname":
		return profile.NameParts.FamilyName
	default:
		return ""
	}
}

func scimFilterMatch(actual, expected, operator string, exact bool) bool {
	if exact {
		switch operator {
		case "eq":
			return actual == expected
		case "sw":
			return strings.HasPrefix(actual, expected)
		default:
			return strings.Contains(actual, expected)
		}
	}
	switch operator {
	case "eq":
		return strings.EqualFold(actual, expected)
	case "sw":
		return strings.HasPrefix(strings.ToLower(actual), strings.ToLower(expected))
	default:
		return strings.Contains(strings.ToLower(actual), strings.ToLower(expected))
	}
}
