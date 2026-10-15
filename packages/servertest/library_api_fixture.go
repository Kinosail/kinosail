package servertest

import (
	"net/http"
	"testing"
)

type LibraryAPIFixture struct {
	NewHandler      func(media, data string, requireAuth bool) http.Handler
	SubtitleLabels  [2]string
	SignIn          func(*testing.T, http.Handler, string, string) *http.Cookie
	Server          func(*testing.T) (http.Handler, string)
	StoredState     func(*testing.T, string, string) []byte
	StoredProfileID func(*testing.T, string, string) string
	NoPlayback      bool
}
