package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestPasskeyLoginSupportsPasswordManagerAutofill(t *testing.T) {
	servertest.PasskeyLoginSupportsPasswordManagerAutofill(t, passkeysJS)
}

func TestCopyControlsReportClipboardFallback(t *testing.T) {
	servertest.CopyControlsReportClipboardFallback(t, themeJS)
}
