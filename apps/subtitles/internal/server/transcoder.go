package server

import "github.com/MikeO7/kinosail/packages/transcodepolicy"

type transcodeSettings = transcodepolicy.Settings

func videoArguments(options transcodeSettings, width string) ([]string, []string) {
	return transcodepolicy.VideoArguments(options, width)
}

func videoCompatibilityArguments(codec string) []string {
	return transcodepolicy.CompatibilityArguments(codec)
}
