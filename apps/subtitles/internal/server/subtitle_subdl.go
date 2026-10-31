package server

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/subtitlelanguage"
)

const defaultSubDLURL = "https://api.subdl.com/api/v1"

type subDLProvider struct {
	url, key string
	client   *providerClient
}

func newSubDLProvider(endpoint, key string, health *subtitleProviderHealthRegistry) *subDLProvider {
	if endpoint == "" {
		endpoint = defaultSubDLURL
	}
	return &subDLProvider{url: endpoint, key: key, client: newProviderClient("SubDL", endpoint, "subdl.com", health)}
}

func (provider *subDLProvider) name() string { return "SubDL" }

func (provider *subDLProvider) configured() bool {
	return validSubtitleProviderEndpoint(provider.url) && validSubtitleProviderCredential(provider.key)
}

func (provider *subDLProvider) candidates(ctx context.Context, item library.Item, language string) ([]subtitleDownloadCandidate, error) {
	found, err := provider.search(ctx, item, language)
	if err != nil {
		return nil, err
	}
	candidates := make([]subtitleDownloadCandidate, 0, len(found))
	for _, candidate := range found {
		link := candidate.URL
		candidates = append(candidates, subtitleDownloadCandidate{Language: language, Score: candidate.Score, ReleaseMatch: candidate.ReleaseMatch, Source: "subdl", HI: candidate.HI, Download: func(ctx context.Context) ([]byte, error) {
			return provider.download(ctx, link)
		}})
	}
	return candidates, nil
}

func (provider *subDLProvider) checkCredentials(ctx context.Context) error {
	endpoint, err := url.Parse(strings.TrimRight(provider.url, "/"))
	if err != nil {
		return err
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/api/v1") + "/api/v2/me"
	query := endpoint.Query()
	query.Set("api_key", provider.key)
	endpoint.RawQuery = query.Encode()
	var response map[string]any
	return provider.client.json(ctx, providerCall{route: "account", method: http.MethodGet, endpoint: endpoint.String()}, &response)
}

func (provider *subDLProvider) search(ctx context.Context, item library.Item, language string) ([]subtitleCandidate, error) {
	providerLanguage, supported := subtitlelanguage.Code(language, subtitlelanguage.SubDL)
	if !supported {
		return nil, errors.New("SubDL does not support this language")
	}
	endpoint, err := url.Parse(strings.TrimRight(provider.url, "/") + "/subtitles")
	if err != nil {
		return nil, errors.New("SubDL is not configured")
	}
	query := endpoint.Query()
	query.Set("api_key", provider.key)
	query.Set("client", "kinosail-subtitles")
	query.Set("file_name", filepath.Base(item.Path))
	query.Set("type", "movie")
	title, year := subtitleSearchIdentity(item)
	ids := item.ProviderIDs
	if item.Show != "" {
		title, year, ids = item.Show, item.ShowYear, item.ShowProviderIDs
		query.Set("type", "tv")
		query.Set("season_number", strconv.Itoa(item.Season))
		query.Set("episode_number", strconv.Itoa(item.Episode))
	}
	query.Set("film_name", title)
	query.Set("hi", "1")
	query.Set("languages", providerLanguage)
	query.Set("releases", "1")
	query.Set("subs_per_page", "30")
	query.Set("unpack", "1")
	if year != "" {
		query.Set("year", year)
	}
	setSubDLIDs(query, ids)
	endpoint.RawQuery = query.Encode()
	var found subDLResponse
	if err := provider.client.json(ctx, providerCall{route: "search", method: http.MethodGet, endpoint: endpoint.String()}, &found); err != nil {
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

func (provider *subDLProvider) download(ctx context.Context, link string) ([]byte, error) {
	data, err := provider.client.file(ctx, providerCall{route: "file", method: http.MethodGet, endpoint: provider.resolve(link)})
	if err != nil {
		return nil, err
	}
	if archive, zipErr := zip.NewReader(bytes.NewReader(data), int64(len(data))); zipErr == nil {
		return readArchivedSubtitle(ctx, archive)
	}
	if !validSubtitlePayload(data) {
		return nil, errors.New("subtitle download is invalid")
	}
	return data, nil
}

func (provider *subDLProvider) resolve(link string) string {
	download, err := url.Parse(link)
	if err != nil || download.IsAbs() {
		return link
	}
	base, err := url.Parse(provider.url)
	if err != nil {
		return link
	}
	base.Path, base.RawQuery = "/", ""
	if strings.HasSuffix(strings.ToLower(base.Hostname()), "subdl.com") {
		base.Scheme, base.Host = "https", "dl.subdl.com"
	}
	return base.ResolveReference(download).String()
}
