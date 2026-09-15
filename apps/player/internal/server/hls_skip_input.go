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
	arguments = append(arguments, "-readrate_initial_burst", "8", "-readrate", "1")
	return append(arguments, "-f", "concat", "-safe", "0", "-i", concat), "1", nil
}
