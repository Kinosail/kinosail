package scim

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/secure/precis"
)

type scimSelection struct {
	included map[string]bool
	excluded map[string]bool
}

type scimQueryOptions struct {
	filter    func(viewerProfile) bool
	start     int
	count     int
	selection scimSelection
}

func scimQueryValues(request *http.Request, allowed ...string) (url.Values, error) {
	if len(request.URL.RequestURI()) > maxSCIMQueryLength {
		return nil, &scimValidationError{detail: "SCIM query is too long"}
	}
	values, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return nil, &scimValidationError{detail: "SCIM query is invalid"}
	}
	allowlist := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowlist[name] = true
	}
	for name, values := range values {
		if !allowlist[name] {
			return nil, &scimValidationError{detail: "SCIM query parameter is not supported"}
		}
		if len(values) != 1 {
			return nil, &scimValidationError{detail: "SCIM query parameter must appear once"}
		}
	}
	return values, nil
}

func scimListOptions(request *http.Request) (scimQueryOptions, error) { //nolint:cyclop // List controls each have independent strict bounds.
	values, err := scimQueryValues(request, "filter", "startIndex", "count", "attributes", "excludedAttributes")
	if err != nil {
		return scimQueryOptions{}, err
	}
	if values["attributes"] != nil && values["attributes"][0] == "" || values["excludedAttributes"] != nil && values["excludedAttributes"][0] == "" {
		return scimQueryOptions{}, &scimValidationError{detail: "SCIM attribute selection cannot be empty"}
	}
	filter, err := scimFilter(values.Get("filter"))
	if err != nil {
		return scimQueryOptions{}, err
	}
	start, count, err := scimListPage(values)
	if err != nil {
		return scimQueryOptions{}, err
	}
	selection, err := scimSelectionFrom(values.Get("attributes"), values.Get("excludedAttributes"))
	if err != nil {
		return scimQueryOptions{}, err
	}
	return scimQueryOptions{filter: filter, start: start, count: count, selection: selection}, nil
}

func scimListPage(values url.Values) (int, int, error) { //nolint:cyclop // Pagination parameters have independent strict bounds.
	start, count := 1, 100
	for name, target := range map[string]*int{"startIndex": &start, "count": &count} {
		value, present := values[name]
		if !present {
			continue
		}
		parsed, err := strconv.Atoi(value[0])
		if err != nil {
			return 0, 0, &scimValidationError{detail: name + " is invalid"}
		}
		if name == "startIndex" && (parsed < 1 || parsed > 1_000_000) || name == "count" && (parsed < 0 || parsed > maxSCIMPageSize) {
			return 0, 0, &scimValidationError{detail: name + " is invalid"}
		}
		*target = parsed
	}
	return start, count, nil
}

func scimResourceOptions(request *http.Request) (scimSelection, error) {
	values, err := scimQueryValues(request, "attributes", "excludedAttributes")
	if err != nil {
		return scimSelection{}, err
	}
	if values["attributes"] != nil && values["attributes"][0] == "" || values["excludedAttributes"] != nil && values["excludedAttributes"][0] == "" {
		return scimSelection{}, &scimValidationError{detail: "SCIM attribute selection cannot be empty"}
	}
	return scimSelectionFrom(values.Get("attributes"), values.Get("excludedAttributes"))
}

func scimSelectionFrom(attributes, excluded string) (scimSelection, error) {
	if attributes != "" && excluded != "" {
		return scimSelection{}, &scimValidationError{detail: "attributes and excludedAttributes are mutually exclusive"}
	}
	selection := scimSelection{}
	var err error
	if attributes != "" {
		selection.included, err = scimAttributeSet(attributes)
		if err != nil {
			return scimSelection{}, err
		}
	}
	if excluded != "" {
		selection.excluded, err = scimAttributeSet(excluded)
		if err != nil {
			return scimSelection{}, err
		}
	}
	return selection, nil
}

func scimAttributeSet(raw string) (map[string]bool, error) {
	values := strings.Split(raw, ",")
	if len(values) > 16 {
		return nil, &scimValidationError{detail: "too many SCIM attributes"}
	}
	result := make(map[string]bool, len(values))
	for _, value := range values {
		canonical, err := canonicalSCIMPath(value)
		if err != nil {
			return nil, err
		}
		if canonical == "" || !scimAttributes[canonical] {
			return nil, &scimValidationError{detail: "SCIM attribute is not supported"}
		}
		if result[canonical] {
			return nil, &scimValidationError{detail: "SCIM attribute must appear once"}
		}
		result[canonical] = true
	}
	return result, nil
}

var scimAttributes = map[string]bool{
	"id": true, "schemas": true, "username": true, "externalid": true, "name": true, "name.formatted": true, "name.givenname": true, "name.familyname": true,
	"displayname": true, "active": true, "emails": true, "emails.value": true, "emails.type": true, "emails.primary": true, "meta": true,
	"enterprise": true, "enterprise.employeenumber": true, "enterprise.costcenter": true, "enterprise.organization": true, "enterprise.division": true,
	"enterprise.department": true, "enterprise.manager": true, "enterprise.manager.value": true, "enterprise.manager.$ref": true, "enterprise.manager.displayname": true,
}

func scimPathID(request *http.Request) (string, error) {
	id := request.PathValue("id")
	if id == "" || len(id) > 256 || !utf8.ValidString(id) || strings.ContainsAny(id, "/\\\x00\r\n") {
		return "", &scimValidationError{detail: "SCIM resource id is invalid"}
	}
	return id, nil
}

func prepareUserName(value string) (string, error) {
	value, err := scimString(value, 256, true, "userName")
	if err != nil {
		return "", err
	}
	prepared, err := precis.UsernameCaseMapped.String(value)
	if err != nil {
		return "", &scimValidationError{detail: "userName is invalid"}
	}
	return scimString(prepared, 256, true, "userName")
}

func canonicalSCIMPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	prefix := scimUserSchema + ":"
	enterprisePrefix := scimEnterpriseUser + ":"
	if strings.EqualFold(raw, scimEnterpriseUser) {
		return "enterprise", nil
	}
	if strings.Contains(raw, ":") {
		if len(raw) > len(enterprisePrefix) && strings.EqualFold(raw[:len(enterprisePrefix)], enterprisePrefix) {
			return "enterprise." + strings.ToLower(strings.TrimSpace(raw[len(enterprisePrefix):])), nil
		}
		if len(raw) <= len(prefix) || !strings.EqualFold(raw[:len(prefix)], prefix) {
			return "", &scimValidationError{detail: "SCIM attribute path is not supported"}
		}
		raw = raw[len(prefix):]
	}
	if strings.Contains(raw, ":") {
		return "", &scimValidationError{detail: "SCIM attribute path is not supported"}
	}
	return strings.ToLower(strings.TrimSpace(raw)), nil
}

func canonicalSCIMPatchPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	prefix := scimUserSchema + ":"
	enterprisePrefix := scimEnterpriseUser + ":"
	if len(raw) >= len(prefix) && strings.EqualFold(raw[:len(prefix)], prefix) {
		raw = raw[len(prefix):]
	} else if len(raw) >= len(enterprisePrefix) && strings.EqualFold(raw[:len(enterprisePrefix)], enterprisePrefix) {
		return "enterprise." + strings.ToLower(strings.TrimSpace(raw[len(enterprisePrefix):])), nil
	} else if colon, opening := strings.IndexByte(raw, ':'), strings.IndexByte(raw, '['); colon >= 0 && (opening < 0 || colon < opening) {
		return "", &scimValidationError{detail: "SCIM attribute path is not supported"}
	}
	return strings.ToLower(strings.TrimSpace(raw)), nil
}
