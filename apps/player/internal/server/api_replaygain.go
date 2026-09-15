package server

type apiReplayGain struct {
	TrackDB *float64 `json:"trackDb,omitempty"`
	AlbumDB *float64 `json:"albumDb,omitempty"`
}

func apiReplayGainFor(gain replayGain) *apiReplayGain {
	if !gain.TrackSet && !gain.AlbumSet {
		return nil
	}
	result := &apiReplayGain{}
	if gain.TrackSet {
		result.TrackDB = &gain.Track
	}
	if gain.AlbumSet {
		result.AlbumDB = &gain.Album
	}
	return result
}
