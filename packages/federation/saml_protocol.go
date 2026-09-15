package federation

import (
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/crewjam/saml"
)

func samlProviderEndpoint(metadata *saml.EntityDescriptor) (string, string, error) { //nolint:gocognit // Selection is ordered and fail-closed.
	if metadata != nil {
		for _, binding := range []string{saml.HTTPRedirectBinding, saml.HTTPPostBinding} {
			for _, descriptor := range metadata.IDPSSODescriptors {
				for _, service := range descriptor.SingleSignOnServices {
					endpoint, err := url.Parse(service.Location)
					if service.Binding == binding && err == nil && TrustedURL(endpoint) {
						return binding, service.Location, nil
					}
				}
			}
		}
	}
	return "", "", ErrProviderUnavailable
}

func samlIdentityValue(assertion *saml.Assertion, attribute string) (string, error) { //nolint:cyclop,gocognit // Signed identity selection stays explicit.
	if strings.EqualFold(attribute, "NameID") {
		if assertion.Subject != nil && assertion.Subject.NameID != nil && ValidSubject(assertion.Subject.NameID.Value) {
			return assertion.Subject.NameID.Value, nil
		}
		return "", ErrTokenInvalid
	}
	values := make([]string, 0, 1)
	for _, statement := range assertion.AttributeStatements {
		for _, candidate := range statement.Attributes {
			if candidate.Name != attribute && candidate.FriendlyName != attribute {
				continue
			}
			for _, value := range candidate.Values {
				values = append(values, value.Value)
			}
		}
	}
	if len(values) != 1 || !ValidSubject(values[0]) {
		return "", ErrTokenInvalid
	}
	return values[0], nil
}

func samlAssertionRequestID(assertion *saml.Assertion) string {
	for _, confirmation := range assertion.Subject.SubjectConfirmations {
		if confirmation.SubjectConfirmationData != nil && confirmation.SubjectConfirmationData.InResponseTo != "" {
			return confirmation.SubjectConfirmationData.InResponseTo
		}
	}
	return ""
}

func validSAMLForm(form url.Values) bool {
	response, responseOK := exactlyOne(form["SAMLResponse"], maxSAMLBody, true)
	_, relayOK := exactlyOne(form["RelayState"], 4096, false)
	if !responseOK || !relayOK || response == "" {
		return false
	}
	for key := range form {
		if !slices.Contains([]string{"SAMLResponse", "RelayState"}, key) {
			return false
		}
	}
	return true
}

func exactlyOne(values []string, maximum int, required bool) (string, bool) {
	value := ""
	if len(values) == 1 {
		value = values[0]
	}
	return value, len(values) == 1 && len(value) <= maximum && utf8.ValidString(value) && (!required || value != "")
}

func formEncoded(request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/x-www-form-urlencoded"
}

func samlHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(request *http.Request, _ []*http.Request) error {
		if !TrustedURL(request.URL) {
			return ErrProviderUnavailable
		}
		return nil
	}}
}
