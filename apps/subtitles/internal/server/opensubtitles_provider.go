package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	"github.com/MikeO7/kinosail/packages/library"
)

type OpenSubtitlesConfig struct {
	URL, APIKey, Username, Password string
}

type openSubtitlesProvider struct {
	config       OpenSubtitlesConfig
	client       *http.Client
	mu           sync.Mutex
	token        string
	sessionURL   string
	tokenExpires time.Time
	login        *openSubtitlesSessionFlight
	health       *subtitleProviderHealthRegistry
}

type openSubtitlesSessionFlight struct {
	done            chan struct{}
	token, endpoint string
	err             error
}

type openSubtitlesLogin struct {
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
	Status  int    `json:"status"`
}

type openSubtitlesDownload struct {
	Link string `json:"link"`
}

func newOpenSubtitlesProvider(config OpenSubtitlesConfig) *openSubtitlesProvider {
	if config.URL == "" {
		config.URL = "https://api.opensubtitles.com/api/v1"
	}
	provider := &openSubtitlesProvider{config: config, health: newSubtitleProviderHealthRegistry()}
	provider.client = localIntegrationHTTPClient(15 * time.Second)
	provider.client.CheckRedirect = func(request *http.Request, _ []*http.Request) error {
		if !provider.allowed(request.URL.String()) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return provider
}

func (provider *openSubtitlesProvider) configured() bool {
	return validSubtitleProviderEndpoint(provider.config.URL) && validSubtitleProviderCredential(provider.config.APIKey) && validSubtitleProviderCredential(provider.config.Username) && validSubtitleProviderCredential(provider.config.Password)
}

func validSubtitleProviderCredential(value string) bool {
	if len(value) == 0 || len(value) > 4096 || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	return strings.IndexFunc(value, unicode.IsControl) < 0
}

func (provider *openSubtitlesProvider) search(ctx context.Context, item library.Item, language string) ([]openSubtitlesCandidate, error) {
	providerLanguage, supported := subtitlelanguage.Code(language, subtitlelanguage.OpenSubtitles)
	if !provider.configured() || !supported {
		return nil, errors.New("OpenSubtitles is not configured")
	}
	baseURL, token := provider.cachedSession()
	endpoint, err := provider.endpointAt(baseURL, "subtitles")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("ai_translated", "include")
	query.Set("foreign_parts_only", "exclude")
	query.Set("hearing_impaired", "include")
	query.Set("languages", providerLanguage)
	query.Set("machine_translated", "exclude")
	query.Set("query", strings.ToLower(filepath.Base(item.Path)))
	query.Set("type", "movie")
	if hash, hashErr := openSubtitlesHash(item.Path); hashErr == nil {
		query.Set("moviehash", hash)
	}
	ids, year := item.ProviderIDs, item.Year
	if item.Show != "" {
		ids, year = item.ShowProviderIDs, item.ShowYear
		query.Set("episode_number", strconv.Itoa(item.Episode))
		query.Set("season_number", strconv.Itoa(item.Season))
		query.Set("type", "episode")
		setOpenSubtitleID(query, "parent_imdb_id", ids["imdb"])
		setOpenSubtitleID(query, "parent_tmdb_id", ids["tmdb"])
	} else {
		setOpenSubtitleID(query, "imdb_id", ids["imdb"])
		setOpenSubtitleID(query, "tmdb_id", ids["tmdb"])
	}
	if numeric := numericProviderID(year); numeric != "" {
		query.Set("year", numeric)
	}
	endpoint.RawQuery = query.Encode()
	var response openSubtitlesResponse
	if err := provider.requestJSON(ctx, http.MethodGet, endpoint.String(), nil, token, &response); err != nil {
		return nil, err
	}
	if query.Get("moviehash") == "" {
		for index := range response.Data {
			response.Data[index].Attributes.MoviehashMatch = false
		}
	}
	return rankOpenSubtitlesCandidates(item, language, response), nil
}

func setOpenSubtitleID(query url.Values, key, value string) {
	if value = numericProviderID(value); value != "" {
		query.Set(key, value)
	}
}

func (provider *openSubtitlesProvider) download(ctx context.Context, fileID int64) ([]byte, error) {
	if fileID <= 0 {
		return nil, errors.New("OpenSubtitles file is invalid")
	}
	token, baseURL, err := provider.session(ctx)
	if err != nil {
		return nil, err
	}
	endpoint, err := provider.endpointAt(baseURL, "download")
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{"file_id": fileID, "sub_format": "srt"})
	var response openSubtitlesDownload
	if err := provider.requestJSON(ctx, http.MethodPost, endpoint.String(), body, token, &response); err != nil || len(response.Link) == 0 || len(response.Link) > 2048 {
		return nil, errors.New("OpenSubtitles download failed")
	}
	if !provider.allowed(response.Link) {
		return nil, errors.New("OpenSubtitles download is not trusted")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, response.Link, nil)
	if err != nil {
		return nil, err
	}
	download, err := provider.client.Do(request)
	if err != nil {
		provider.health.observe("OpenSubtitles", nil, err)
		return nil, err
	}
	defer download.Body.Close()
	data, err := readSubtitle(download)
	provider.health.observe("OpenSubtitles", download, err)
	return data, err
}

func (provider *openSubtitlesProvider) session(ctx context.Context) (string, string, error) {
	provider.mu.Lock()
	if provider.token != "" && time.Now().Before(provider.tokenExpires) {
		token, endpoint := provider.token, provider.sessionURL
		provider.mu.Unlock()
		return token, endpoint, nil
	}
	if flight := provider.login; flight != nil {
		provider.mu.Unlock()
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-flight.done:
			return flight.token, flight.endpoint, flight.err
		}
	}
	flight := &openSubtitlesSessionFlight{done: make(chan struct{})}
	provider.login = flight
	provider.mu.Unlock()
	flight.token, flight.endpoint, flight.err = provider.loginSession(ctx)
	provider.mu.Lock()
	if flight.err == nil {
		provider.token, provider.sessionURL, provider.tokenExpires = flight.token, flight.endpoint, time.Now().Add(11*time.Hour)
	}
	provider.login = nil
	close(flight.done)
	provider.mu.Unlock()
	return flight.token, flight.endpoint, flight.err
}

func (provider *openSubtitlesProvider) loginSession(ctx context.Context) (string, string, error) {
	endpoint, err := provider.endpoint("login")
	if err != nil {
		return "", "", err
	}
	body, _ := json.Marshal(map[string]string{"username": provider.config.Username, "password": provider.config.Password})
	var login openSubtitlesLogin
	if err := provider.requestJSON(ctx, http.MethodPost, endpoint.String(), body, "", &login); err != nil {
		return "", "", err
	}
	if login.Status != http.StatusOK || !validSubtitleProviderCredential(login.Token) {
		provider.health.observe("OpenSubtitles", nil, errors.New("invalid login response"))
		return "", "", errors.New("OpenSubtitles login failed")
	}
	if login.BaseURL != "" && login.BaseURL != "api.opensubtitles.com" && login.BaseURL != "vip-api.opensubtitles.com" {
		provider.health.observe("OpenSubtitles", nil, errors.New("invalid login host"))
		return "", "", errors.New("OpenSubtitles login host is invalid")
	}
	base := provider.config.URL
	if login.BaseURL != "" {
		base = "https://" + login.BaseURL + "/api/v1"
	}
	return login.Token, base, nil
}

func (provider *openSubtitlesProvider) cachedSession() (string, string) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.token != "" && time.Now().Before(provider.tokenExpires) {
		return provider.sessionURL, provider.token
	}
	return provider.config.URL, ""
}

func (provider *openSubtitlesProvider) requestJSON(ctx context.Context, method, endpoint string, body []byte, token string, target any) error {
	if !provider.allowed(endpoint) {
		return errors.New("OpenSubtitles endpoint is not trusted")
	}
	if err := provider.health.before("OpenSubtitles"); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Api-Key", provider.config.APIKey)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Kinosail Subtitles v0.1")
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := provider.client.Do(request)
	if err != nil {
		provider.health.observe("OpenSubtitles", nil, err)
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		provider.health.observe("OpenSubtitles", response, errors.New("request failed"))
		return errors.New("OpenSubtitles rejected request")
	}
	err = decodeExternalJSON(response.Body, 2<<20, target)
	provider.health.observe("OpenSubtitles", response, err)
	return err
}

func (provider *openSubtitlesProvider) endpoint(path string) (*url.URL, error) {
	return provider.endpointAt(provider.config.URL, path)
}

func (provider *openSubtitlesProvider) endpointAt(baseURL, path string) (*url.URL, error) {
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/" + path)
	if err != nil || !provider.allowed(endpoint.String()) {
		return nil, errors.New("OpenSubtitles endpoint is invalid")
	}
	return endpoint, nil
}

func (provider *openSubtitlesProvider) allowed(link string) bool {
	base, baseErr := url.Parse(provider.config.URL)
	target, targetErr := url.Parse(link)
	if baseErr != nil || targetErr != nil || target.Host == "" {
		return false
	}
	if target.Host == base.Host {
		return target.Scheme == base.Scheme
	}
	host := strings.ToLower(target.Hostname())
	return target.Scheme == "https" && (host == "opensubtitles.com" || strings.HasSuffix(host, ".opensubtitles.com"))
}
