package server

import (
	"strconv"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

type transcodeSettings = transcodepolicy.Settings

func videoArguments(options transcodeSettings, width string) ([]string, []string) {
	return transcodepolicy.VideoArguments(options, width)
}

func videoCompatibilityArguments(codec string) []string {
	return transcodepolicy.CompatibilityArguments(codec)
}

func presentationFilterGraph(options transcodeSettings, recipe hlsRecipe, qualities []PlaybackQuality) string {
	if recipe.burn != "" || len(recipe.omitted) > 0 {
		return ""
	}
	widths := make([]string, len(qualities))
	for index, quality := range qualities {
		widths[index] = strconv.Itoa(quality.Width)
	}
	return transcodepolicy.PresentationFilterGraph(options, widths)
}
