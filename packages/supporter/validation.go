package supporter

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	keyPattern          = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)
	activationIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	supporterIDPattern  = regexp.MustCompile(`^[0-9A-F]{10}$`)
	appIDPattern        = regexp.MustCompile(`^[a-z](?:[a-z0-9-]{0,30}[a-z0-9])?$`)
	editionPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 -]{0,23}$`)
	timePattern         = regexp.MustCompile(`^[2-9][0-9]{3}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?Z$`)
	tiers               = []string{"friend", "crew", "navigator", "patron", "steward", "lighthouse", "commodore", "admiral", "northstar", "legacy"}
)

// ValidKey reports whether a normalized activation key follows Player's bounded alphabet.
func ValidKey(value string) bool { return keyPattern.MatchString(value) }

// Tiers returns an isolated copy of Player's ordered supporter levels.
func Tiers() []string { return slices.Clone(tiers) }

// ValidEndpoint applies Player's remote endpoint policy.
func ValidEndpoint(value string, allowLoopbackHTTP bool) bool { //nolint:cyclop // Endpoint trust rules are clearer as one predicate.
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(parsed.Hostname())
	return allowLoopbackHTTP && parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || ip != nil && ip.IsLoopback())
}

// ValidRecognitionName checks an optional signed public name.
func ValidRecognitionName(value string, requireValue bool) bool {
	if value == "" {
		return !requireValue
	}
	if len(value) > 80 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}

// Rank maps Player's canonical tier names to levels.
func Rank(tier string) int { return slices.Index(tiers, tier) + 1 }

// Name returns Player's canonical display name for a tier.
func Name(tier string) string {
	if tier == "" || tier == "free" {
		return "Free"
	}
	if tier == "northstar" {
		return "North Star"
	}
	if Rank(tier) == 0 {
		return "Free"
	}
	return strings.ToUpper(tier[:1]) + tier[1:]
}

func subscriptionName(tier string) string {
	return map[string]string{"watch": "Watch", "voyage": "Voyage", "harbor": "Harbor", "fleet": "Fleet", "legacy": "Legacy"}[tier]
}

func decodeValue(value string, size, maximum int) ([]byte, bool) {
	if value == "" || len(value) > maximum || strings.Contains(value, "=") {
		return nil, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return decoded, err == nil && base64.RawURLEncoding.EncodeToString(decoded) == value && (size < 0 || len(decoded) == size)
}

func strictTime(value string) (time.Time, bool) {
	if !timePattern.MatchString(value) {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return parsed, err == nil
}

func hashKey(key string) string {
	digest := sha256.Sum256([]byte(key))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func supporterMark(installationKey string) string {
	digest := sha256.Sum256([]byte(installationKey))
	return strings.ToUpper(hex.EncodeToString(digest[:5]))
}
