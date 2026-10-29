package server

import "github.com/MikeO7/kinosail/packages/hlsmanifest"

func hlsSkipInput(arguments []string, directory, path string, duration float64, recipe hlsRecipe) ([]string, string, error) {
	concat, err := hlsmanifest.WriteSkipConcat(directory, path, duration, recipe.omitted)
	if err != nil {
		return nil, "", err
	}
	if recipe.offset > 0 {
		arguments = append(arguments, "-ss", ffmpegSeconds(recipe.offset))
	}
	return append(arguments, "-f", "concat", "-safe", "0", "-i", concat), "1", nil
}

func hlsSkippedTailInput(arguments []string, duration, start float64, window hlsRecipe) []string {
	remaining := duration - start
	for _, omitted := range window.omitted {
		if remaining > 0 && omitted.End >= remaining-0.5 {
			return append(arguments, "-to", ffmpegSeconds(start+min(remaining, omitted.Start+1)))
		}
	}
	return arguments
}
