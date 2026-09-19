package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
	"golang.org/x/text/language"
)

func validate(spec Spec, raw string) error {
	err := validateKind(spec.Kind, raw)
	if err == nil {
		if validator := configurationValidators[spec.Key]; validator != nil {
			err = validator(raw)
		}
	}
	if err != nil {
		return fmt.Errorf("invalid %s: %w", spec.Key, err)
	}
	return nil
}

func validateKind(kind kind, raw string) error {
	switch kind {
	case text:
		return nil
	case boolean:
		if raw != "" {
			_, err := strconv.ParseBool(raw)
			return err
		}
	case duration:
		_, err := time.ParseDuration(raw)
		return err
	case number:
		value, err := strconv.Atoi(raw)
		if err == nil && value < 1 {
			return errors.New("must be positive")
		}
		return err
	case list:
		var values []string
		return json.Unmarshal([]byte(raw), &values)
	}
	return nil
}

var configurationValidators = map[string]func(string) error{
	"server.name":                          validateServerName,
	"tls.hosts":                            validateTLSHosts,
	"tls.duckdns":                          func(raw string) error { _, err := trustedhttps.Parse(raw); return err },
	"playback.mode":                        allowed("", "automatic", "direct", "compatible"),
	"remote.mode":                          allowed("off", "https"),
	"remote.duckdns_domain":                optionalValid(validDNSLabel, "must be one DNS label"),
	"remote.duckdns_token":                 optionalValid(validRemoteToken, "must contain 32 to 128 safe characters"),
	"integrations.scim.token":              optionalValid(validSCIMToken, "must contain 32 to 256 non-whitespace characters"),
	"integrations.scim.token_expires_at":   validateSCIMExpiration,
	"integrations.oidc.issuer":             validateIdentityURL,
	"integrations.oidc.redirect_url":       validateIdentityURL,
	"integrations.saml.metadata_url":       validateIdentityURL,
	"integrations.oidc.client_id":          optionalValid(func(raw string) bool { return federation.BoundedIdentityText(raw, 512) }, "must contain no more than 512 valid characters"),
	"integrations.oidc.client_secret":      optionalValid(func(raw string) bool { return federation.BoundedIdentityText(raw, 4096) }, "must contain no more than 4096 valid characters"),
	"integrations.oidc.identity_claim":     requiredValid(federation.ValidIdentityField, "must contain 1 to 256 non-whitespace characters"),
	"integrations.saml.identity_attribute": requiredValid(federation.ValidIdentityField, "must contain 1 to 256 non-whitespace characters"),
	"integrations.saml.metadata_xml":       validateSAMLDocument,
	"logging.level":                        allowed("debug", "info", "warn", "error"),
	"playback.subtitles":                   allowed("", "on", "off"),
	"transcoding.quality":                  allowed("", "automatic", "speed", "quality"),
	"transcoding.codec":                    allowed("", "auto", "h264", "hevc", "av1", "vp9"),
	"transcoding.accelerator":              requiredValid(ValidTranscodingAccelerator, "unsupported value"),
	"scanning.frequency":                   allowed("", "default", "off", "5m", "15m", "1h"),
	"subtitles.language":                   validateSubtitleLanguage,
	"logging.audit_retention":              validatePositiveDuration,
	"logging.playback_retention":           validatePositiveDuration,
}

func allowed(values ...string) func(string) error {
	return func(raw string) error { return oneOf(raw, values...) }
}

func optionalValid(valid func(string) bool, message string) func(string) error {
	return func(raw string) error {
		if raw != "" && !valid(raw) {
			return errors.New(message)
		}
		return nil
	}
}

func requiredValid(valid func(string) bool, message string) func(string) error {
	return func(raw string) error {
		if !valid(raw) {
			return errors.New(message)
		}
		return nil
	}
}

func validateServerName(raw string) error {
	if raw == "" || len(raw) > 64 {
		return errors.New("must contain 1 to 64 characters")
	}
	return nil
}

func validateTLSHosts(raw string) error {
	var hosts []string
	_ = json.Unmarshal([]byte(raw), &hosts)
	if len(raw) > 16<<10 || len(hosts) > 32 {
		return errors.New("must contain no more than 32 hosts")
	}
	for _, host := range hosts {
		if !validTLSHost(host) {
			return errors.New("must contain only hostnames or IP addresses without ports")
		}
	}
	return nil
}

func validateSCIMExpiration(raw string) error {
	if raw != "" {
		if _, err := time.Parse(time.RFC3339, raw); err != nil {
			return errors.New("must be an RFC3339 time")
		}
	}
	return nil
}

func validateIdentityURL(raw string) error {
	if raw != "" && (len(raw) > 2048 || !federation.ValidIdentityURL(raw)) {
		return errors.New("must be a trusted identity provider URL")
	}
	return nil
}

func validateSAMLDocument(raw string) error {
	if len(raw) > 256<<10 || !federation.ValidSAMLMetadata(raw) {
		return errors.New("must be a valid SAML identity provider metadata document no larger than 256 KiB")
	}
	return nil
}

func validateSubtitleLanguage(raw string) error {
	if _, err := language.ParseBase(strings.ToLower(raw)); raw != "" && err != nil {
		return errors.New("must be a two or three letter code")
	}
	return nil
}

func validatePositiveDuration(raw string) error {
	if value, _ := time.ParseDuration(raw); value <= 0 {
		return errors.New("must be positive")
	}
	return nil
}

// ValidTranscodingAccelerator reports whether every configuration and settings adapter accepts value.
func ValidTranscodingAccelerator(value string) bool {
	return oneOf(value, "", "none", "auto", "vaapi", "qsv", "cuda", "videotoolbox", "rkmpp", "v4l2m2m", "amf", "mf") == nil
}

func validTLSHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if host == "" || len(host) > 253 || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	for label := range strings.SplitSeq(host, ".") {
		if !validDNSLabel(label) {
			return false
		}
	}
	return true
}

func validDNSLabel(label string) bool {
	if label == "" || len(label) > 63 || !letterOrDigit(label[0]) || !letterOrDigit(label[len(label)-1]) {
		return false
	}
	for index := 1; index < len(label)-1; index++ {
		if !letterOrDigit(label[index]) && label[index] != '-' {
			return false
		}
	}
	return true
}

func letterOrDigit(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
}

func oneOf(value string, allowed ...string) error {
	if slices.Contains(allowed, value) {
		return nil
	}
	return errors.New("unsupported value")
}
