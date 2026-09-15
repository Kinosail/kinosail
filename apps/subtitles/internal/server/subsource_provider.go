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
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
)

type SubSourceConfig struct {
	URL, APIKey string
	PersonalUse bool
}

type subSourceProvider struct {
	config SubSourceConfig
	client *http.Client
	health *subtitleProviderHealthRegistry
}

func newSubSourceProvider(config SubSourceConfig) *subSourceProvider {
	if config.URL == "" {
		config.URL = "https://api.subsource.net/api/v1"
	}
	provider := &subSourceProvider{config: config, client: localIntegrationHTTPClient(15 * time.Second), health: newSubtitleProviderHealthRegistry()}
	provider.client.CheckRedirect = func(request *http.Request, _ []*http.Request) error {
		if !provider.allowed(request.URL.String()) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return provider
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
	var movies subSourceMovieResponse
	if err = provider.requestJSON(ctx, endpoint.String(), &movies); err != nil {
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
	var subtitles subSourceSubtitleResponse
	if err = provider.requestJSON(ctx, endpoint.String(), &subtitles); err != nil {
		return nil, err
	}
	return rankSubSourceCandidates(item, movie, requestedLanguage, subtitles), nil
}

func (provider *subSourceProvider) download(ctx context.Context, candidate subSourceCandidate, item library.Item) ([]byte, error) {
	if err := provider.health.before("SubSource"); err != nil {
		return nil, err
	}
	endpoint, err := provider.endpoint("subtitles/" + strconv.FormatInt(candidate.ID, 10) + "/download")
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	provider.headers(request)
	response, err := provider.client.Do(request)
	if err != nil {
		provider.health.observe("SubSource", nil, err)
		return nil, err
	}
	defer response.Body.Close()
	data, err := readSubtitle(response)
	if err != nil {
		provider.health.observe("SubSource", response, err)
		return nil, err
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		err = errors.New("SubSource download is not a ZIP archive")
		provider.health.observe("SubSource", response, err)
		return nil, err
	}
	data, err = readSubSourceArchive(archive, item)
	provider.health.observe("SubSource", response, err)
	return data, err
}

func readSubSourceArchive(archive *zip.Reader, item library.Item) ([]byte, error) { //nolint:cyclop,gocognit // Archive selection validates every candidate before reading one bounded subtitle.
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
		if readErr == nil && len(data) <= 4<<20 && utf8.Valid(data) && validSubtitlePayload(data) {
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

func (provider *subSourceProvider) requestJSON(ctx context.Context, endpoint string, target any) error {
	if !provider.allowed(endpoint) {
		return errors.New("SubSource endpoint is not trusted")
	}
	if err := provider.health.before("SubSource"); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	provider.headers(request)
	response, err := provider.client.Do(request)
	if err != nil {
		provider.health.observe("SubSource", nil, err)
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		provider.health.observe("SubSource", response, errors.New("request failed"))
		return errors.New("SubSource rejected request")
	}
	err = decodeExternalJSON(response.Body, 2<<20, target)
	provider.health.observe("SubSource", response, err)
	return err
}

func (provider *subSourceProvider) headers(request *http.Request) {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Kinosail Subtitles v0.1")
	request.Header.Set("X-API-Key", provider.config.APIKey)
}

func (provider *subSourceProvider) endpoint(path string) (*url.URL, error) {
	endpoint, err := url.Parse(strings.TrimRight(provider.config.URL, "/") + "/" + path)
	if err != nil || !provider.allowed(endpoint.String()) {
		return nil, errors.New("SubSource endpoint is invalid")
	}
	return endpoint, nil
}

func (provider *subSourceProvider) allowed(link string) bool {
	base, baseErr := url.Parse(provider.config.URL)
	target, targetErr := url.Parse(link)
	return baseErr == nil && targetErr == nil && target.Host != "" && target.Host == base.Host && target.Scheme == base.Scheme
}
