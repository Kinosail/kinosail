package remoteaccess

import (
	"errors"
	"strings"
	"testing"
)

func TestValidTokenRejectsUnsafeAndOutOfRangeValues(t *testing.T) {
	for name, token := range map[string]string{
		"too short":   strings.Repeat("a", 31),
		"too long":    strings.Repeat("a", 129),
		"space":       strings.Repeat("a", 31) + " ",
		"unicode":     strings.Repeat("a", 31) + "é",
		"punctuation": strings.Repeat("a", 31) + ".",
	} {
		t.Run(name, func(t *testing.T) {
			if validToken(token) {
				t.Fatalf("validToken(%q) accepted unsafe input", token)
			}
		})
	}
	if !validToken(strings.Repeat("aA0-_", 7)) {
		t.Fatal("validToken rejected the supported token alphabet")
	}
}

func TestValidDomainRejectsUnsafeAndOutOfRangeValues(t *testing.T) {
	for name, domain := range map[string]string{
		"empty":           "",
		"leading hyphen":  "-media",
		"trailing hyphen": "media-",
		"uppercase":       "Media",
		"unicode":         "média",
		"too long":        strings.Repeat("a", 64),
	} {
		t.Run(name, func(t *testing.T) {
			if validDomain(domain) {
				t.Fatalf("validDomain(%q) accepted unsafe input", domain)
			}
		})
	}
	if !validDomain("family-media-2") {
		t.Fatal("validDomain rejected the supported domain alphabet")
	}
}

func TestSetStatusRecordsErrorsButDoesNotReviveKilledManager(t *testing.T) {
	manager := &Manager{}
	manager.setStatus("error", errors.New("connection failed"))
	if manager.status.State != "error" || manager.status.Error != "connection failed" {
		t.Fatalf("status = %#v", manager.status)
	}
	manager.killed = true
	manager.setStatus("ready", nil)
	if manager.status.State != "error" || manager.status.Error != "connection failed" {
		t.Fatalf("killed manager status changed = %#v", manager.status)
	}
}
