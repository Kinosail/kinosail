package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStepUpReturnsToKnownOwnerPage(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://media.example/settings/encrypted-backup", nil)
	if got, want := stepUpLoginPath(request), "/login?stepup=1&next=%2Fsettings%2Fbackups"; got != want {
		t.Fatalf("step-up path = %q, want %q", got, want)
	}
	request.URL.Path = "/settings/profiles"
	if got := stepUpLoginPath(request); got != "/login?stepup=1&next=%2Fsettings" {
		t.Fatalf("settings step-up path = %q", got)
	}
	request.URL.Path = "/unknown/action"
	if got := stepUpLoginPath(request); got != "/login?stepup=1&next=%2F" {
		t.Fatalf("unknown step-up path = %q", got)
	}
}

func TestLoginReturnRejectsExternalAndAmbiguousTargets(t *testing.T) {
	for _, raw := range []string{"https://attacker.example", "//attacker.example", `/\\attacker.example`, "%2F%2Fattacker.example", strings.Repeat("a", 2049)} {
		if got := safeLoginReturn(raw); got != "/" {
			t.Errorf("safeLoginReturn(%q) = %q", raw, got)
		}
	}
	if got := safeLoginReturn("/settings/backups?from=status"); got != "/settings/backups?from=status" {
		t.Fatalf("valid return = %q", got)
	}
}

func TestPasskeyOfferPathKeepsTheValidatedLoginDestination(t *testing.T) {
	if got := passkeyOfferPath("/settings/backups?from=status"); got != "/account?passkey=offer&next=%2Fsettings%2Fbackups%3Ffrom%3Dstatus" {
		t.Fatalf("passkey offer path = %q", got)
	}
}
