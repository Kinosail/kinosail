package remoteaccess

import (
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestSetupNeverClaimsOutsideReachability(t *testing.T) {
	now := time.Now()
	for _, status := range []Status{
		{},
		{Mode: "https", State: "ready", Policy: "public-v1", CertificateExpires: now.Add(48 * time.Hour).Format(time.RFC3339)},
	} {
		result := SecurePublicReadiness(status, []identitycore.Profile{{Remote: true, Passkeys: []webauthn.Credential{{}}}}, nil, now)
		if result.Reachability != "unverified" {
			t.Fatal("local checks claimed outside reachability")
		}
		if len(result.NextSteps) < 4 || result.NextSteps[len(result.NextSteps)-1].Title != "Try it away from home" {
			t.Fatalf("missing outside verification: %#v", result.NextSteps)
		}
		if result.Ready && len(result.NextSteps) != 4 {
			t.Fatal("passed checks still show configuration recovery")
		}
	}
}

func TestSetupExplainsFailedChecksAndPortMapping(t *testing.T) {
	result := SecurePublicReadiness(Status{}, nil, nil, time.Now())
	var instructions strings.Builder
	for _, step := range result.NextSteps {
		instructions.WriteString(step.Title + " " + step.Instruction + "\n")
	}
	joined := instructions.String()
	for _, required := range []string{"Enable public HTTPS", "Prepare a Viewer Profile", "internal port 443", "container 8443", "cellular data", "do not change your router or firewall automatically", "Set up an authenticator app", "Quick Connect"} {
		if !strings.Contains(joined, required) {
			t.Errorf("missing guidance: %s", required)
		}
	}
	if strings.Index(joined, "Add one port-forwarding rule") > strings.Index(joined, "Finish the certificate check") {
		t.Fatal("certificate instructions precede their network prerequisite")
	}
}

func TestStoppedSetupDoesNotTellOwnersToReopenRouterPorts(t *testing.T) {
	result := SecurePublicReadiness(Status{Mode: "https", State: "killed"}, nil, nil, time.Now())
	if !result.Stopped || result.Ready || len(result.NextSteps) != 1 || !strings.Contains(result.NextSteps[0].Instruction, "Keep the router rule disabled") {
		t.Fatalf("stopped setup = %#v", result)
	}
}

func TestSetupShowsExactRouterValuesAndOfficialGuides(t *testing.T) {
	result := SecurePublicReadiness(Status{}, nil, nil, time.Now())
	for _, step := range result.NextSteps {
		if len(step.Fields) == 0 {
			continue
		}
		if len(step.Fields) != 5 || step.Fields[2].Value != "TCP only" || step.Fields[3].Value != "443" || step.Fields[4].Value != "443" {
			t.Fatalf("unsafe router mapping: %#v", step.Fields)
		}
		if len(step.Links) != 6 {
			t.Fatalf("missing manufacturer guides: %#v", step.Links)
		}
		return
	}
	t.Fatal("no router field mapping")
}

func TestSetupPublicAddressRejectsUntrustedHostnames(t *testing.T) {
	for _, hostname := range []string{"", "https://family.duckdns.org", "family.duckdns.org.evil.test", "family.duckdns.org:443", "family.duckdns.org/path", "family.duckdns.org@evil.test", "family.duckdns.org?x=1", "family.duckdns.org#x", "family..duckdns.org", "-family.duckdns.org", "family-.duckdns.org", "fámily.duckdns.org", "Family.duckdns.org", strings.Repeat("a", 64) + ".duckdns.org", "family.duckdns.org\n"} {
		result := SecurePublicReadiness(Status{Mode: "https", Hostname: hostname}, nil, nil, time.Now())
		if result.PublicURL != "" {
			t.Errorf("untrusted hostname %q became a setup link: %q", hostname, result.PublicURL)
		}
	}
	for _, mode := range []string{"off", "https"} {
		result := SecurePublicReadiness(Status{Mode: mode, Hostname: "family.duckdns.org"}, nil, nil, time.Now())
		if mode == "https" && result.PublicURL != "https://family.duckdns.org" || mode != "https" && result.PublicURL != "" {
			t.Errorf("%s setup public URL = %q", mode, result.PublicURL)
		}
	}
}

func TestSetupAcceptsSecureQuickConnectAndRejectsDisabledViewers(t *testing.T) {
	now := time.Now()
	status := Status{Mode: "https", State: "ready", Policy: "public-v1", CertificateExpires: now.Add(48 * time.Hour).Format(time.RFC3339)}
	for name, profile := range map[string]identitycore.Profile{
		"authenticator": {Remote: true, TOTPSecret: "test-factor"},
		"disabled":      {Remote: true, Disabled: true, TOTPSecret: "test-factor"},
		"owner":         {Remote: true, Owner: true, TOTPSecret: "test-factor"},
		"local":         {TOTPSecret: "test-factor"},
		"password only": {Remote: true},
	} {
		result := SecurePublicReadiness(status, []identitycore.Profile{profile}, nil, now)
		if result.Ready != (name == "authenticator") {
			t.Errorf("%s readiness = %v", name, result.Ready)
		}
	}
}
