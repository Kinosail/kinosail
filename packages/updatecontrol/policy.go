package updatecontrol

type DocumentStore interface {
	LoadJSON(string, any) (bool, error)
	SaveJSON(string, any) error
}

type Policy struct {
	tagPrefix           string
	signatureIdentity   string
	stateSchema         int
	configurationSchema int
}

func PlayerPolicy(stateSchema, configurationSchema int) Policy {
	return releasePolicy("player", stateSchema, configurationSchema)
}

func SubtitlesPolicy(stateSchema, configurationSchema int) Policy {
	return releasePolicy("subtitles", stateSchema, configurationSchema)
}
