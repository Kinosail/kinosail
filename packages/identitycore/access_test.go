package identitycore

import (
	"net/http"
	"testing"
)

func allowedAccess() AccessInput {
	return AccessInput{RecentlyAuthenticated: true, PublicSession: true, RemoteRouteAllowed: true, ScheduleAllowed: true, APIKeyAllowed: true, LocalOwner: true, Secured: true, EnrollmentRoute: true}
}

func TestEvaluateAccessPrecedence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*AccessInput)
		want   Denial
	}{
		{"allowed", func(*AccessInput) {}, Allowed},
		{"remote Owner first", func(input *AccessInput) { input.Public, input.Owner, input.RecentlyAuthenticated = true, true, false }, OwnerRemoteDenied},
		{"remote API key first", func(input *AccessInput) { input.Public, input.APIKey, input.RecentlyAuthenticated = true, true, false }, OwnerRemoteDenied},
		{"strong authentication", func(input *AccessInput) { input.Public, input.RecentlyAuthenticated = true, false }, StrongAuthenticationRequired},
		{"public session", func(input *AccessInput) { input.Public, input.PublicSession = true, false }, PublicSessionRequired},
		{"remote route", func(input *AccessInput) { input.Public, input.RemoteRouteAllowed = true, false }, RemoteRouteDenied},
		{"schedule", func(input *AccessInput) { input.ScheduleAllowed = false }, ViewerScheduleDenied},
		{"API scope", func(input *AccessInput) { input.APIKey, input.APIKeyAllowed = true, false }, APIKeyScopeDenied},
		{"API scope allowed", func(input *AccessInput) { input.APIKey = true }, Allowed},
		{"MFA required", func(input *AccessInput) {
			input.LocalOwner, input.MFARequired, input.Secured, input.EnrollmentRoute = false, true, false, false
		}, MFAEnrollmentRequired},
		{"Owner MFA required", func(input *AccessInput) {
			input.LocalOwner, input.Owner, input.Secured, input.EnrollmentRoute = false, true, false, false
		}, MFAEnrollmentRequired},
		{"local Owner exemption", func(input *AccessInput) { input.Owner, input.Secured, input.EnrollmentRoute = true, false, false }, Allowed},
		{"secured exemption", func(input *AccessInput) { input.LocalOwner, input.Owner, input.EnrollmentRoute = false, true, false }, Allowed},
		{"enrollment exemption", func(input *AccessInput) { input.LocalOwner, input.MFARequired, input.Secured = false, true, false }, Allowed},
		{"unmanaged private Viewer", func(input *AccessInput) { input.LocalOwner, input.Secured, input.EnrollmentRoute = false, false, false }, Allowed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := allowedAccess()
			test.change(&input)
			if got := EvaluateAccess(input); got != test.want {
				t.Fatalf("EvaluateAccess(%+v) = %v, want %v", input, got, test.want)
			}
		})
	}
}

func TestOwnerPolicyAndDenialPresentation(t *testing.T) { //nolint:cyclop // The denial presentation table covers every stable denial value.
	t.Parallel()
	if EvaluateOwner(false, true, true, true, true) != OwnerRequired {
		t.Fatal("non-Owner was allowed")
	}
	if EvaluateOwner(true, false, false, false, false) != OwnerStepUpRequired {
		t.Fatal("remote Owner without step-up was allowed")
	}
	for _, allowed := range []Denial{
		EvaluateOwner(true, true, false, false, false),
		EvaluateOwner(true, false, true, false, false),
		EvaluateOwner(true, false, false, true, false),
		EvaluateOwner(true, false, false, false, true),
	} {
		if allowed != Allowed {
			t.Fatalf("Owner exemption denied: %v", allowed)
		}
	}
	tests := map[Denial]struct {
		reason, message string
		status          int
	}{
		OwnerRemoteDenied:            {"Owner and API key access unavailable remotely", "Owner access is available only on the local network or WireGuard", http.StatusForbidden},
		StrongAuthenticationRequired: {"strong authentication required remotely", "strong authentication is required for public access", http.StatusForbidden},
		PublicSessionRequired:        {"public session required remotely", "public sign-in is required for public access", http.StatusForbidden},
		RemoteRouteDenied:            {"route unavailable remotely", "Owner access required", http.StatusNotFound},
		ViewerScheduleDenied:         {"viewer access unavailable", "Viewer access is not available", http.StatusForbidden},
		APIKeyScopeDenied:            {"API key scope", "API key scope does not allow this request", http.StatusForbidden},
		MFAEnrollmentRequired:        {"MFA enrollment required", "MFA enrollment required", http.StatusForbidden},
		OwnerRequired:                {"Owner access required", "Owner access required", http.StatusForbidden},
		OwnerStepUpRequired:          {"Owner access required", "Owner access required", http.StatusForbidden},
		Allowed:                      {"Owner access required", "Owner access required", http.StatusNotFound},
	}
	for denial, want := range tests {
		if denial.Reason() != want.reason || denial.Message() != want.message || denial.Status() != want.status {
			t.Fatalf("denial %v = %q, %q, %d", denial, denial.Reason(), denial.Message(), denial.Status())
		}
	}
	unknown := Denial(255)
	if unknown.Reason() != "Owner access required" || unknown.Message() != "Owner access required" || unknown.Status() != http.StatusNotFound {
		t.Fatalf("unknown denial = %q, %q, %d", unknown.Reason(), unknown.Message(), unknown.Status())
	}
}

func TestMFAAndAPIKeyPolicies(t *testing.T) {
	t.Parallel()
	if Secured("", 0) || !Secured("secret", 0) || !Secured("", 1) {
		t.Fatal("secured profile policy changed")
	}
	for _, route := range []string{"GET /account", "POST /account/mfa/setup", "POST /account/mfa/enable", "POST /auth/passkeys/register/begin", "POST /auth/passkeys/register/finish", "POST /logout", "DELETE /api/v1/session", "GET /api/v1/me", "POST /api/v1/me/mfa/setup", "PUT /api/v1/me/mfa", "POST /api/v1/passkeys/register/begin", "POST /api/v1/passkeys/register/finish"} {
		if !MFAEnrollmentRoute(route) {
			t.Fatalf("enrollment route rejected: %q", route)
		}
	}
	if MFAEnrollmentRoute("POST /api/v1/me/mfa/disable") {
		t.Fatal("unknown enrollment route was allowed")
	}
	routes := APIRoutes{
		SessionOnly:     RouteSet("POST /api/v1/session"),
		JellyfinLibrary: RouteSet("GET /api/v1/jellyfin/libraries"),
		Library:         RouteSet("GET /api/v1/library"),
		Write:           RouteSet("POST /api/v1/library"),
		Stream:          RouteSet("GET /api/v1/stream"),
		Download:        RouteSet("GET /api/v1/download"),
	}
	tests := []struct {
		scopes  []string
		pattern string
		allowed bool
	}{
		{[]string{"home-assistant"}, "GET /api/v1/home-assistant/status", true},
		{[]string{"home-assistant"}, "GET /api/v1/me", false},
		{[]string{"admin"}, "POST /api/v1/session", false},
		{[]string{"library"}, "GET /api/v1/me", true},
		{[]string{"admin"}, "PUT /api/v1/me/language", true},
		{[]string{"library"}, "GET /api/v1/jellyfin/libraries", true},
		{[]string{"admin"}, "GET /api/v1/jellyfin/libraries", false},
		{[]string{"library"}, "GET /api/v1/library", true},
		{[]string{"write"}, "POST /api/v1/library", true},
		{[]string{"stream"}, "GET /api/v1/stream", true},
		{[]string{"download"}, "GET /api/v1/download", true},
		{[]string{"admin"}, "DELETE /api/v1/other", true},
		{[]string{"admin"}, "GET /settings", false},
		{nil, "GET /api/v1/library", false},
	}
	for _, test := range tests {
		if got := AllowsAPI(test.scopes, test.pattern, routes); got != test.allowed {
			t.Fatalf("AllowsAPI(%v, %q) = %v", test.scopes, test.pattern, got)
		}
	}
	set := RouteSet("one", "two")
	set["one"] = false
	if !RouteSet("one")["one"] {
		t.Fatal("route sets shared mutable state")
	}
}
