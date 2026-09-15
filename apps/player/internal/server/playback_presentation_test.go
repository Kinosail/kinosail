package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestPlaybackPresentationNamesEveryTransformPrecisely(t *testing.T) {
	servertest.PlaybackPresentationNamesEveryTransformPrecisely(t, playbackPresentation)
}
