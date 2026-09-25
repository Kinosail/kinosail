package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestApplicationPagesExposeSharedKeyboardShortcuts(t *testing.T) {
	servertest.ApplicationPagesExposeSharedKeyboardShortcuts(t, settingsSearchHandler, "30", `!activeMenu?.contains(event.target)`)
}
