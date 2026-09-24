package playback

import (
	"errors"
	"strings"
)

func parseRecipeAudioEffects(parts []string) ([]string, bool, bool, error) {
	last := parts[len(parts)-1]
	if !strings.HasPrefix(last, "e") {
		return parts, false, false, nil
	}
	effects, err := prefixedInt(last, "e")
	if err != nil || effects < 1 || effects > 3 {
		return nil, false, false, errors.New("audio effects are invalid")
	}
	return parts[:len(parts)-1], effects&1 != 0, effects&2 != 0, nil
}
