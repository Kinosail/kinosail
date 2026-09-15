package catalog

import "github.com/MikeO7/kinosail/packages/library"

// PlayAction describes the canonical next Show episode action.
type PlayAction struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Label  string `json:"label"`
	Stream string `json:"stream"`
}

// ShowPlay selects the first unwatched episode or starts the Show again.
func ShowPlay(show library.Show, progress func(string) PlaybackState) *PlayAction {
	for _, episode := range show.Episodes {
		state := progress(episode.ID)
		if !state.Watched {
			label := "Play next"
			if state.Seconds > 0 {
				label = "Resume"
			}
			return &PlayAction{episode.ID, episode.Title, label, "/watch/" + episode.ID}
		}
	}
	if len(show.Episodes) == 0 {
		return nil
	}
	episode := show.Episodes[0]
	return &PlayAction{episode.ID, episode.Title, "Play again", "/watch/" + episode.ID}
}
