package updatecontrol

// DocumentStore is the bounded persistence contract required by update state.
type DocumentStore interface {
	LoadJSON(string, any) (bool, error)
	SaveJSON(string, any) error
}

// Policy keeps application release identity and schema compatibility explicit.
type Policy struct {
	tagPrefix           string
	signatureIdentity   string
	stateSchema         int
	configurationSchema int
}

// PlayerPolicy returns Player's release boundary for the supplied app schemas.
func PlayerPolicy(stateSchema, configurationSchema int) Policy {
	return releasePolicy("player", stateSchema, configurationSchema)
}

// SubtitlesPolicy returns Subtitles' independent release boundary.
func SubtitlesPolicy(stateSchema, configurationSchema int) Policy {
	return releasePolicy("subtitles", stateSchema, configurationSchema)
}
