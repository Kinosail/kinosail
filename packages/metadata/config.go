package metadata

const (
	defaultAPIURL   = "https://api.themoviedb.org/3"
	defaultImageURL = "https://image.tmdb.org/t/p/w780"
)

// Config describes optional TMDB, chapter, and TVMaze providers.
type Config struct{ URL, ImageURL, Token, ChaptersURL, TVMazeURL string }

// NormalizeConfig fills the stable public TMDB endpoints.
func NormalizeConfig(config Config) Config {
	if config.URL == "" {
		config.URL = defaultAPIURL
	}
	if config.ImageURL == "" {
		config.ImageURL = defaultImageURL
	}
	return config
}
