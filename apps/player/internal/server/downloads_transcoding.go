package server

import (
	"github.com/MikeO7/kinosail/packages/downloads"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func downloadTranscoding(settings *settingsStore) func(bool) transcodepolicy.Settings {
	return downloads.Transcoding(func() transcodepolicy.Settings { options, _ := settings.transcodingFor("h264"); return options }, func(codec string) string {
		return settings.hardware.Backend("none").EncoderFor(codec)
	})
}
