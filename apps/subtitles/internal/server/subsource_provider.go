package server

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type SubSourceConfig struct {
	URL, APIKey string
	PersonalUse bool
}

type subSourceProvider struct {
	config    SubSourceConfig
	client    *providerClient
	now       func() time.Time
	movies    subSourceCache[subSourceMovieResponse]
	subtitles subSourceCache[subSourceSubtitleResponse]
}

func newSubSourceProvider(config SubSourceConfig, health *subtitleProviderHealthRegistry) *subSourceProvider {
	if config.URL == "" {
		config.URL = "https://api.subsource.net/api/v1"
	}
	return &subSourceProvider{config: config, client: newProviderClient("SubSource", config.URL, "", health), now: time.Now}
}

func (provider *subSourceProvider) name() string { return "SubSource" }

func (provider *subSourceProvider) checkCredentials(ctx context.Context) error {
	endpoint, err := provider.endpoint("languages")
	if err != nil {
		return err
	}
	var response map[string]any
	return provider.requestJSON(ctx, "languages", endpoint.String(), &response)
}

func (provider *subSourceProvider) configured() bool {
	return provider.config.PersonalUse && validSubtitleProviderCredential(provider.config.APIKey) && validSubtitleProviderEndpoint(provider.config.URL)
}

func (provider *subSourceProvider) search(ctx context.Context, item library.Item, language string) ([]subSourceCandidate, error) {
	requestedLanguage := language
	language, ok := subSourceLanguage(language)
	if !provider.configured() || !ok {
		return nil, errors.New("SubSource is not configured for this language")
	}
	endpoint, err := provider.endpoint("movies/search")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	title, year := subtitleSearchIdentity(item)
	ids := item.ProviderIDs
	if item.Show != "" {
		ids, title, year = item.ShowProviderIDs, item.Show, item.ShowYear
		query.Set("season", strconv.Itoa(item.Season))
		query.Set("type", "series")
	} else {
		query.Set("type", "movie")
	}
	if imdb := ids["imdb"]; imdb != "" {
		query.Set("searchType", "imdb")
		query.Set("imdb", imdb)
	} else {
		query.Set("searchType", "text")
		query.Set("q", title)
	}
	endpoint.RawQuery = query.Encode()
	movies, err := cachedSubSourceJSON(ctx, provider, &provider.movies, "movies", endpoint.String())
	if err != nil {
		return nil, err
	}
	movie, ok := selectSubSourceMovie(item, ids, title, year, movies)
	if !ok {
		return nil, nil
	}
	endpoint, err = provider.endpoint("subtitles")
	if err != nil {
		return nil, err
	}
	query = endpoint.Query()
	query.Set("language", language)
	query.Set("limit", "50")
	query.Set("movieId", strconv.FormatInt(movie.MovieID, 10))
	query.Set("sort", "rating")
	endpoint.RawQuery = query.Encode()
	subtitles, err := cachedSubSourceJSON(ctx, provider, &provider.subtitles, "subtitles", endpoint.String())
	if err != nil {
		return nil, err
	}
	return rankSubSourceCandidates(item, movie, requestedLanguage, subtitles), nil
}

func (provider *subSourceProvider) download(ctx context.Context, candidate subSourceCandidate, item library.Item) ([]byte, error) {
	endpoint, err := provider.endpoint("subtitles/" + strconv.FormatInt(candidate.ID, 10) + "/download")
	if err != nil {
		return nil, err
	}
	data, err := provider.client.file(ctx, providerCall{route: "file", method: http.MethodGet, endpoint: endpoint.String(), header: provider.header()})
	if err != nil {
		return nil, err
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("SubSource download is not a ZIP archive")
	}
	return readSubSourceArchive(archive, item)
}

func readSubSourceArchive(archive *zip.Reader, item library.Item) ([]byte, error) {
	if len(archive.File) == 0 || len(archive.File) > 100 {
		return nil, errors.New("SubSource archive has invalid cardinality")
	}
	var subtitle []byte
	for _, file := range archive.File {
		cleanName := path.Clean(strings.ReplaceAll(file.Name, "\\", "/"))
		if len(file.Name) > 512 || path.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, "../") || file.FileInfo().IsDir() || strings.ToLower(filepath.Ext(cleanName)) != ".srt" || file.UncompressedSize64 > 4<<20 || item.Show != "" && !subSourceEpisodeName(filepath.Base(cleanName), item.Season, item.Episode) {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, (4<<20)+1))
		_ = reader.Close()
		if readErr == nil && len(data) <= 4<<20 && validSubtitlePayload(data) {
			if subtitle != nil {
				return nil, errors.New("SubSource archive is ambiguous")
			}
			subtitle = data
		}
	}
	if subtitle == nil {
		return nil, errors.New("SubSource archive has no exact SRT match")
	}
	return subtitle, nil
}

func (provider *subSourceProvider) requestJSON(ctx context.Context, route, endpoint string, target any) error {
	return provider.client.json(ctx, providerCall{route: route, method: http.MethodGet, endpoint: endpoint, header: provider.header()}, target)
}

func (provider *subSourceProvider) header() http.Header {
	header := http.Header{}
	header.Set("X-API-Key", provider.config.APIKey)
	return header
}

func (provider *subSourceProvider) endpoint(path string) (*url.URL, error) {
	return provider.client.endpoint(provider.config.URL, path)
}
