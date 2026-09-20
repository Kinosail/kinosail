package identitycore

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStepUpLoginPathReturnsToKnownOwnerPage(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"/settings/management/users", "/login?stepup=1&next=%2Fsettings%2Fmanagement"},
		{"/settings/backup", "/login?stepup=1&next=%2Fsettings%2Fbackups"},
		{"/settings/encrypted-backup", "/login?stepup=1&next=%2Fsettings%2Fbackups"},
		{"/settings/backups/verify", "/login?stepup=1&next=%2Fsettings%2Fbackups"},
		{"/settings/profiles", "/login?stepup=1&next=%2Fsettings"},
		{"/account/security", "/login?stepup=1&next=%2Faccount"},
		{"/unknown/action", "/login?stepup=1&next=%2F"},
	}
	for _, test := range tests {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://media.example"+test.path, nil)
		if got := StepUpLoginPath(request); got != test.want {
			t.Errorf("StepUpLoginPath(%q) = %q, want %q", test.path, got, test.want)
		}
	}
	if got := StepUpLoginPath(nil); got != "/login?stepup=1&next=%2F" {
		t.Fatalf("StepUpLoginPath(nil) = %q", got)
	}
}

func TestSafeLoginReturnRejectsExternalAndAmbiguousTargets(t *testing.T) {
	for _, raw := range []string{"", "https://attacker.example", "//attacker.example", `/\\attacker.example`, "%2F%2Fattacker.example", strings.Repeat("a", 2049), "/account#" + strings.Repeat("a", 2049), "//attacker.example#local"} {
		if got := SafeLoginReturn(raw); got != "/" {
			t.Errorf("SafeLoginReturn(%q) = %q", raw, got)
		}
	}
	for raw, want := range map[string]string{"/settings/backups?from=status": "/settings/backups?from=status", "/account#passkeys": "/account", "/account%23passkeys": "/account%23passkeys", "/account?next=home#passkeys": "/account?next=home"} {
		if got := SafeLoginReturn(raw); got != want {
			t.Errorf("SafeLoginReturn(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestPasskeyOfferPathEscapesValidatedDestination(t *testing.T) {
	if got := PasskeyOfferPath("/settings/backups?from=status"); got != "/account?passkey=offer&next=%2Fsettings%2Fbackups%3Ffrom%3Dstatus" {
		t.Fatalf("PasskeyOfferPath = %q", got)
	}
}
