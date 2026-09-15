package server

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	"github.com/MikeO7/kinosail/packages/library"
)

func (provider *subtitleProvider) searchSubDL(ctx context.Context, item library.Item, language string) ([]subtitleCandidate, error) {
	providerLanguage, supported := subtitlelanguage.Code(language, subtitlelanguage.SubDL)
	if !supported {
		return nil, errors.New("SubDL does not support this language")
	}
	endpoint, err := url.Parse(strings.TrimRight(provider.config.URL, "/") + "/subtitles")
	if err != nil {
		return nil, errors.New("SubDL is not configured")
	}
	query := endpoint.Query()
	query.Set("api_key", provider.config.APIKey)
	query.Set("client", "kinosail-subtitles")
	query.Set("file_name", filepath.Base(item.Path))
	title, year := subtitleSearchIdentity(item)
	query.Set("film_name", title)
	query.Set("hi", "1")
	query.Set("languages", providerLanguage)
	query.Set("releases", "1")
	query.Set("subs_per_page", "30")
	query.Set("type", "movie")
	query.Set("unpack", "1")
	if year != "" {
		query.Set("year", year)
	}
	setSubDLIDs(query, item.ProviderIDs)
	if item.Show != "" {
		query.Set("film_name", item.Show)
		query.Set("season_number", strconv.Itoa(item.Season))
		query.Set("episode_number", strconv.Itoa(item.Episode))
		query.Set("type", "tv")
		setSubDLIDs(query, item.ShowProviderIDs)
	}
	endpoint.RawQuery = query.Encode()
	var found subDLResponse
	if err := provider.json(ctx, endpoint.String(), &found); err != nil {
		return nil, errors.New("SubDL search failed")
	}
	return rankSubDLCandidates(item, language, found), nil
}

func setSubDLIDs(query url.Values, ids map[string]string) {
	for provider, id := range ids {
		if oneOf(provider, "imdb", "tmdb") && id != "" {
			query.Set(provider+"_id", id)
		}
	}
}
