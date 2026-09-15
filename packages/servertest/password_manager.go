package servertest

import (
	"strings"
	"testing"
)

// PasswordFormsExposeStablePasswordManagerSemantics runs the corresponding app regression contract.
func PasswordFormsExposeStablePasswordManagerSemantics(t *testing.T, setupHTML, profileLoginHTML, settingsHTML string, ignoreNonPasswordSecretAutofill func(string) string) {
	t.Parallel()
	tests := []struct {
		name string
		page string
		want []string
	}{
		{"Owner setup", setupHTML, []string{
			`id="setup-username" autofocus required name="name" maxlength="64" autocomplete="username"`,
			`id="new-password" required type="password" name="password" minlength="12" autocomplete="new-password"`,
		}},
		{"Sign in", profileLoginHTML, []string{
			`id="username" autofocus required name="name" autocomplete="username webauthn"`,
			`id="current-password" required type="password" name="password" autocomplete="current-password"`,
			`name="code" autocomplete="one-time-code"`,
		}},
		{"Profile management", ignoreNonPasswordSecretAutofill(settingsHTML), []string{
			`id="new-profile-username" aria-label="New profile name" name="name"`,
			`autocomplete="username" autocapitalize="none" spellcheck="false" required`,
			`id="new-profile-password" aria-label="New profile password" name="password" type="password" placeholder="Password" minlength="12" autocomplete="new-password"`,
			`id="reset-profile-password" aria-label="New profile password" name="password" type="password" placeholder="New password" minlength="12" autocomplete="new-password"`,
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, want := range test.want {
				if !strings.Contains(test.page, want) {
					t.Errorf("page missing %q", want)
				}
			}
		})
	}
}

// AuthenticationCodeFieldsUseLocalizedNumericSemantics runs the corresponding app regression contract.
func AuthenticationCodeFieldsUseLocalizedNumericSemantics(t *testing.T, mfaSetupHTML, profileLoginHTML, accountHTML string) {
	t.Parallel()
	if !strings.Contains(mfaSetupHTML, `{{t "Authentication code"}}`) {
		t.Error("MFA setup authentication code label is not localized")
	}
	if !strings.Contains(mfaSetupHTML, `name="code" autocomplete="one-time-code" inputmode="numeric" pattern="[0-9]{6}" maxlength="6"`) {
		t.Error("MFA setup code input does not expose its six-digit format")
	}
	for _, test := range []struct {
		name string
		page string
	}{
		{"Sign in", profileLoginHTML},
		{"Account", accountHTML},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertRecoveryCodeField(t, test.page)
		})
	}
	if !strings.Contains(profileLoginHTML, `{{t "if enabled"}}`) {
		t.Error("sign-in authentication helper is not localized")
	}
}

func assertRecoveryCodeField(t *testing.T, page string) {
	t.Helper()
	if !strings.Contains(page, `{{t "Authentication or recovery code"}}`) {
		t.Error("authentication or recovery code label is not localized")
	}
	start := strings.Index(page, `name="code"`)
	if start < 0 {
		t.Fatal("authentication or recovery code input is missing")
	}
	end := strings.Index(page[start:], ">")
	if end < 0 {
		t.Fatal("authentication or recovery code input is missing")
	}
	field := page[start : start+end]
	if strings.Contains(field, "inputmode") || strings.Contains(field, "pattern") || strings.Contains(field, "maxlength") {
		t.Errorf("recovery code input has numeric-only constraints: %s", field)
	}
}

// NonPasswordSecretsOptOutOfPasswordManagers runs the corresponding app regression contract.
func NonPasswordSecretsOptOutOfPasswordManagers(t *testing.T, configurationHTML, settingsHTML, onboardingConnectionHTML, viewingImportOnboardingHTML string, ignoreNonPasswordSecretAutofill func(string) string) {
	t.Parallel()
	const ignored = `autocomplete="off" data-1p-ignore data-bwignore data-lpignore="true" data-form-type="other" spellcheck="false"`
	for _, test := range []struct {
		name string
		page string
		want int
	}{
		{"Deployment configuration", configurationHTML, 1},
		{"Settings", settingsHTML, 2},
		{"Connection onboarding", onboardingConnectionHTML, 1},
		{"Viewing import onboarding", viewingImportOnboardingHTML, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := strings.Count(ignoreNonPasswordSecretAutofill(test.page), ignored); got != test.want {
				t.Errorf("ignored secret fields = %d, want %d", got, test.want)
			}
		})
	}
}

// PasskeyLoginSupportsPasswordManagerAutofill runs the corresponding app regression contract.
func PasskeyLoginSupportsPasswordManagerAutofill(t *testing.T, passkeysJS []byte) {
	t.Parallel()
	script := string(passkeysJS)
	for _, want := range []string{
		`PublicKeyCredential.isConditionalMediationAvailable()`,
		`mediation: "conditional"`,
		`new AbortController()`,
		`conditionalLogin?.abort()`,
		`location.replace(loginNext)`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("passkey script missing %q", want)
		}
	}
}

// PasswordFieldsOfferAccessibleRevealControls runs the corresponding app regression contract.
func PasswordFieldsOfferAccessibleRevealControls(t *testing.T, themeJS, appCSS []byte) {
	t.Parallel()
	for _, want := range []string{
		`input[type="password"]`,
		`button.type = "button"`,
		`button.setAttribute("aria-label", "Show secret")`,
		`button.setAttribute("aria-pressed", "false")`,
		`input.type = shown ? "password" : "text"`,
		`button.setAttribute("aria-label", shown ? "Show secret" : "Hide secret")`,
	} {
		if !strings.Contains(string(themeJS), want) {
			t.Errorf("password controls script missing %q", want)
		}
	}
	for _, want := range []string{".password-control", ".password-toggle", ".password-toggle[aria-pressed=true]"} {
		if !strings.Contains(string(appCSS), want) {
			t.Errorf("password controls styles missing %q", want)
		}
	}
}

// CopyControlsReportClipboardFallback runs the corresponding app regression contract.
func CopyControlsReportClipboardFallback(t *testing.T, themeJS []byte) {
	t.Parallel()
	script := string(themeJS)
	for _, want := range []string{`[data-copy-target]`, `navigator.clipboard.writeText`, `[data-copy-success]`, `|| "Copied."`, `input.select()`, `[data-copy-fallback]`, `|| "Select Copy in your browser."`} {
		if !strings.Contains(script, want) {
			t.Errorf("copy controls script missing %q", want)
		}
	}
}

// DisclosureLinksOpenAndFocusTheirTarget runs the corresponding app regression contract.
func DisclosureLinksOpenAndFocusTheirTarget(t *testing.T, themeJS []byte) {
	t.Parallel()
	script := string(themeJS)
	for _, want := range []string{`[data-open-disclosure]`, `instanceof HTMLAnchorElement`, `instanceof HTMLDetailsElement`, `event.preventDefault()`, `disclosure.open = true`, `location.hash === link.hash`, `scrollIntoView`, `focus({ preventScroll: true })`} {
		if !strings.Contains(script, want) {
			t.Errorf("disclosure controls script missing %q", want)
		}
	}
}
