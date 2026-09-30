package mcpgateway

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

const eventSubscriptionLimit = 256

type eventSubscription struct {
	ID               string    `json:"id"`
	Principal        Principal `json:"principal"`
	Scope            string    `json:"scope"`
	Grant            string    `json:"grant,omitempty"`
	Remote           bool      `json:"remote,omitempty"`
	AuthorityExpires int64     `json:"authorityExpires,omitempty"`
	Name             string    `json:"name"`
	Resource         string    `json:"resource,omitempty"`
	URL              string    `json:"url"`
	Secret           string    `json:"secret"`
	OldSecret        string    `json:"oldSecret,omitempty"`
	RotateUntil      int64     `json:"rotateUntil,omitempty"`
	Expires          int64     `json:"expires"`
}

func eventKind(name string) (string, string, bool) {
	switch name {
	case "library.updated":
		return "The indexed media library changed.", "/api/v1/library", false
	case "download.updated":
		return "A download owned by the connected Viewer Profile changed.", "/api/v1/downloads/", false
	case "home-assistant.command":
		return "A Home Assistant player command changed for the connected Viewer Profile.", "/api/v1/home-assistant/players/", false
	case "subtitles.updated":
		return "Subtitle acquisition, edit, replacement, or Hide state changed. Owner management access is required.", "/api/v1/subtitle-library", true
	}
	return "", "", false
}

func eventResource(name, resource string) bool {
	_, prefix, _ := eventKind(name)
	if prefix == "" || len(resource) > 2048 {
		return false
	}
	if resource == "" {
		return true
	}
	if strings.HasSuffix(prefix, "/") {
		return strings.HasPrefix(resource, prefix) && len(resource) > len(prefix) && !strings.ContainsAny(strings.TrimPrefix(resource, prefix), "/?#%\\")
	}
	return resource == prefix
}

func eventURL(value string) (string, error) { //nolint:cyclop // Every URL component is checked before any outbound side effect.
	if len(value) == 0 || len(value) > 2048 || strings.ContainsAny(value, "\r\n\t ") {
		return "", errors.New("invalid callback URL")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", errors.New("invalid callback URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || !strings.Contains(host, ".") && net.ParseIP(host) == nil {
		return "", errors.New("invalid callback host")
	}
	if ip := net.ParseIP(host); ip != nil && !identitycore.AllowedOutboundIP(ip) {
		return "", errors.New("prohibited callback address")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", errors.New("invalid callback port")
		}
	}
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String(), nil
}

func eventSecret(value string) ([]byte, error) {
	if len(value) > 94 || !strings.HasPrefix(value, "whsec_") {
		return nil, errors.New("invalid signing secret")
	}
	key, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(value, "whsec_"))
	if err != nil || len(key) < 24 || len(key) > 64 {
		return nil, errors.New("invalid signing secret")
	}
	return key, nil
}

func eventTTL(value json.RawMessage) (time.Duration, error) {
	if len(value) == 0 || string(value) == "null" {
		return 24 * time.Hour, nil
	}
	var ttl int64
	if json.Unmarshal(value, &ttl) != nil || ttl <= 0 || ttl > 365*24*60*60*1000 {
		return 0, errors.New("invalid subscription lifetime")
	}
	return max(time.Minute, min(time.Duration(ttl)*time.Millisecond, 24*time.Hour)), nil
}

func eventID(subscription eventSubscription) string {
	identity, _ := json.Marshal([]string{subscription.Scope, subscription.URL, subscription.Name, subscription.Resource})
	hash := sha256.Sum256(identity)
	return "sub_" + hex.EncodeToString(hash[:])
}

func validateEventParams(params *eventParams, unsubscribe bool) error { //nolint:cyclop,gocognit // The boundary validates all cross-field invariants before callbacks.
	if params == nil || params.Arguments == nil || params.Name == "" || len(params.Name) > 128 || len(params.Arguments) > 1 {
		return eventError(-32602, "invalid event name or arguments", nil)
	}
	if _, prefix, _ := eventKind(params.Name); prefix == "" {
		return eventError(-32011, "event was not found", map[string]string{"kind": "event"})
	}
	if !eventResource(params.Name, params.Arguments["resource"]) {
		return eventError(-32602, "invalid event resource", nil)
	}
	for key := range params.Arguments {
		if key != "resource" {
			return eventError(-32602, "unknown event argument", nil)
		}
	}
	if params.Delivery.Mode != "webhook" {
		return eventError(-32014, "unsupported event delivery mode", map[string]string{"feature": "deliveryMode"})
	}
	normalized, err := eventURL(params.Delivery.URL)
	if err != nil {
		return eventError(-32602, "invalid callback URL", nil)
	}
	params.Delivery.URL = normalized
	if params.Cursor != nil || len(params.MaxAge) > 0 {
		return eventError(-32014, "event replay is unavailable", map[string]string{"feature": "replay"})
	}
	if unsubscribe {
		if params.Delivery.Secret != "" || len(params.TTL) > 0 {
			return eventError(-32602, "unsubscribe cannot change signing secret or lifetime", nil)
		}
		return nil
	}
	if _, err := eventSecret(params.Delivery.Secret); err != nil {
		return eventError(-32602, "invalid signing secret", nil)
	}
	if _, err := eventTTL(params.TTL); err != nil {
		return eventError(-32602, "invalid subscription lifetime", nil)
	}
	return nil
}

func validEventState(subscriptions map[string]eventSubscription) bool { //nolint:cyclop // Every persisted record is bounded and validated before creating authority.
	if len(subscriptions) > eventSubscriptionLimit {
		return false
	}
	for id, subscription := range subscriptions {
		callback, err := eventURL(subscription.URL)
		if err != nil || callback != subscription.URL || subscription.ID != id || id != eventID(subscription) || subscription.Principal.ID == "" || len(subscription.Principal.ID) > 256 || len(subscription.Scope) > 4096 || len(subscription.Grant) > 128 || subscription.Expires <= 0 || !validEventAuthority(subscription) || !eventResource(subscription.Name, subscription.Resource) {
			return false
		}
		if _, err := eventSecret(subscription.Secret); err != nil {
			return false
		}
		if subscription.OldSecret != "" {
			if _, err := eventSecret(subscription.OldSecret); err != nil || subscription.RotateUntil <= 0 {
				return false
			}
		}
	}
	return true
}

func validEventAuthority(subscription eventSubscription) bool { //nolint:cyclop // Persisted authority must match exactly one transport-specific identity.
	profile := subscription.Principal.ID
	switch {
	case subscription.Scope == "host:"+profile:
		return subscription.Principal.Owner && subscription.Grant == "" && subscription.AuthorityExpires == 0 && !subscription.Remote
	case subscription.Grant != "":
		return subscription.Scope == "grant:"+subscription.Grant+":"+profile && subscription.AuthorityExpires == 0
	case strings.HasPrefix(subscription.Scope, "external:") && strings.HasSuffix(subscription.Scope, ":"+profile):
		client := strings.TrimSuffix(strings.TrimPrefix(subscription.Scope, "external:"), ":"+profile)
		return client != "" && validEventClientID(client) && subscription.AuthorityExpires > 0 && subscription.Expires <= subscription.AuthorityExpires
	}
	return false
}

func validEventClientID(client string) bool {
	return len(client) <= 2048 && !strings.ContainsAny(client, "\r\n\x00")
}
