package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestEmbeddedTextCaptionsAreSemanticAndExtractedThroughEveryAdapter(t *testing.T) {
	fixture := servertest.EmbeddedSubtitleFixture{
		New: automaticSkipFixture.New, SignIn: signInTestProfile, Login: jellyfinLogin,
		WebCaptionLabel: "English · Captions",
		WebCall:         requestWithCookie, Call: jellyfinCall, Decode: decodeJellyfin,
	}
	fixture.TextCaptionsAcrossAdapters(t)
}
