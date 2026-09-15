package federation

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/oauth2"
)

type oidcProviderMetadata interface {
	Claims(any) error
	Endpoint() oauth2.Endpoint
}

func validateOIDCProvider(provider oidcProviderMetadata) error {
	var metadata struct {
		JWKSURL string `json:"jwks_uri"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return ErrProviderUnavailable
	}
	endpoint := provider.Endpoint()
	for _, raw := range []string{endpoint.AuthURL, endpoint.TokenURL, metadata.JWKSURL} {
		parsed, err := url.Parse(raw)
		if err != nil || !TrustedURL(parsed) {
			return ErrProviderUnavailable
		}
	}
	return nil
}

// TrustedURL accepts HTTPS endpoints and explicit loopback HTTP endpoints.
func TrustedURL(endpoint *url.URL) bool {
	return endpoint != nil && endpoint.Host != "" && endpoint.User == nil && endpoint.RawQuery == "" && endpoint.Fragment == "" && (endpoint.Scheme == "https" || endpoint.Scheme == "http" && (endpoint.Hostname() == "127.0.0.1" || endpoint.Hostname() == "localhost"))
}

// ValidSubject reports whether a provider identity is safe to store and compare.
func ValidSubject(subject string) bool {
	if subject == "" || len(subject) > 255 || !utf8.ValidString(subject) {
		return false
	}
	for _, character := range subject {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func parseOIDCCallbackQuery(rawQuery string) (string, string, bool, bool) {
	if len(rawQuery) > 16<<10 {
		return "", "", false, false
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil || !validOIDCCallbackValues(values) {
		return "", "", false, false
	}
	responseIssuer, valid := oidcResponseIssuer(values)
	if !valid {
		return "", "", false, false
	}
	if code, present, valid := oidcAuthorizationCode(values); present {
		return code, responseIssuer, false, valid
	}
	return "", responseIssuer, true, validOIDCErrorResponse(values)
}

func validOIDCCallbackValues(values url.Values) bool {
	if len(values) > 16 || !validOIDCQueryValue(values["state"], 256) {
		return false
	}
	for key, entries := range values {
		if !validOAuthParameterName(key) || !validOIDCExtensionValue(entries, 4096) {
			return false
		}
	}
	return true
}

func oidcResponseIssuer(values url.Values) (string, bool) {
	issuer, present := values["iss"]
	if !present {
		return "", true
	}
	if !validOIDCQueryValue(issuer, 2048) {
		return "", false
	}
	return issuer[0], true
}

func oidcAuthorizationCode(values url.Values) (string, bool, bool) {
	code, present := values["code"]
	if !present {
		return "", false, false
	}
	_, hasError := values["error"]
	_, hasDescription := values["error_description"]
	_, hasErrorURI := values["error_uri"]
	valid := validOIDCQueryValue(code, 4096) && !hasError && !hasDescription && !hasErrorURI
	if !valid {
		return "", true, false
	}
	return code[0], true, true
}

func validOIDCErrorResponse(values url.Values) bool {
	if !validOAuthErrorValue(values["error"], 256) {
		return false
	}
	for key, entry := range values {
		switch key {
		case "state", "error", "iss":
		case "error_description":
			if !validOAuthErrorValue(entry, 2048) {
				return false
			}
		case "error_uri":
			if !validOIDCQueryValue(entry, 4096) {
				return false
			}
		}
	}
	return true
}

func parseState(rawQuery string) string {
	values, _ := url.ParseQuery(rawQuery)
	return values.Get("state")
}

func validOAuthParameterName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("-.0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz", character) {
			return false
		}
	}
	return true
}

func validOIDCExtensionValue(values []string, maximum int) bool {
	if len(values) != 1 || len(values[0]) > maximum || !utf8.ValidString(values[0]) {
		return false
	}
	for _, character := range values[0] {
		if character < ' ' || character == 0x7f {
			return false
		}
	}
	return true
}

func validOIDCQueryValue(values []string, maximum int) bool {
	return len(values) == 1 && values[0] != "" && validOIDCExtensionValue(values, maximum)
}

func validOAuthErrorValue(values []string, maximum int) bool {
	if len(values) != 1 || values[0] == "" || len(values[0]) > maximum {
		return false
	}
	for _, character := range values[0] {
		if character < 0x20 || character > 0x7e || character == '"' || character == '\\' {
			return false
		}
	}
	return true
}

func tokenExtra(token *oauth2.Token, err error) (string, bool) {
	if err != nil || token == nil {
		return "", false
	}
	raw, ok := token.Extra("id_token").(string)
	return raw, ok && raw != ""
}

func tokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func sameState(left, right string) bool {
	return left != "" && right != "" && len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func boundedURLText(value string) bool {
	return value != "" && len(value) <= 2048 && utf8.ValidString(value)
}

func prune[K comparable, V any](values map[K]V, now time.Time, expires func(V) time.Time) {
	for key, value := range values {
		if !now.Before(expires(value)) {
			delete(values, key)
		}
	}
}
