package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestApplicationPagesExposeSharedKeyboardShortcuts(t *testing.T) {
	servertest.ApplicationPagesExposeSharedKeyboardShortcuts(t, settingsSearchHandler, "13", "pendingGo")
}
