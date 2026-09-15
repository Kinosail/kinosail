package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestPasswordFormsExposeStablePasswordManagerSemantics(t *testing.T) {
	servertest.PasswordFormsExposeStablePasswordManagerSemantics(t, setupHTML, profileLoginHTML, settingsHTML, ignoreNonPasswordSecretAutofill)
}

func TestAuthenticationCodeFieldsUseLocalizedNumericSemantics(t *testing.T) {
	servertest.AuthenticationCodeFieldsUseLocalizedNumericSemantics(t, mfaSetupHTML, profileLoginHTML, accountHTML)
}

func TestNonPasswordSecretsOptOutOfPasswordManagers(t *testing.T) {
	servertest.NonPasswordSecretsOptOutOfPasswordManagers(t, configurationHTML, settingsHTML, onboardingConnectionHTML, viewingImportOnboardingHTML, ignoreNonPasswordSecretAutofill)
}

func TestPasskeyLoginSupportsPasswordManagerAutofill(t *testing.T) {
	servertest.PasskeyLoginSupportsPasswordManagerAutofill(t, passkeysJS)
}

func TestPasswordFieldsOfferAccessibleRevealControls(t *testing.T) {
	servertest.PasswordFieldsOfferAccessibleRevealControls(t, themeJS, appCSS)
}

func TestCopyControlsReportClipboardFallback(t *testing.T) {
	servertest.CopyControlsReportClipboardFallback(t, themeJS)
}

func TestDisclosureLinksOpenAndFocusTheirTarget(t *testing.T) {
	servertest.DisclosureLinksOpenAndFocusTheirTarget(t, themeJS)
}
