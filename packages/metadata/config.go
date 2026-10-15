package metadata

const (
	defaultAPIURL   = "https://api.themoviedb.org/3"
	defaultImageURL = "https://image.tmdb.org/t/p/w780"
)

type Config struct{ URL, ImageURL, Token, ChaptersURL, TVMazeURL string }

func NormalizeConfig(config Config) Config {
	if config.URL == "" {
		config.URL = defaultAPIURL
	}
	if config.ImageURL == "" {
		config.ImageURL = defaultImageURL
	}
	return config
}
