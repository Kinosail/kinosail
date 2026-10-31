package server

import (
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

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/subtitlelanguage"
)

type OpenSubtitlesConfig struct {
	URL, APIKey, Username, Password string
}

type openSubtitlesProvider struct {
	config       OpenSubtitlesConfig
	mu           sync.Mutex
	token        string
	sessionURL   string
	tokenExpires time.Time
	login        *openSubtitlesSessionFlight
	client       *providerClient
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

func newOpenSubtitlesProvider(config OpenSubtitlesConfig, health *subtitleProviderHealthRegistry) *openSubtitlesProvider {
	if config.URL == "" {
		config.URL = "https://api.opensubtitles.com/api/v1"
	}
	return &openSubtitlesProvider{config: config, client: newProviderClient("OpenSubtitles", config.URL, "opensubtitles.com", health)}
}

func (provider *openSubtitlesProvider) name() string { return "OpenSubtitles" }

func (provider *openSubtitlesProvider) checkCredentials(ctx context.Context) error {
	_, _, err := provider.session(ctx)
	return err
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
	endpoint, err := provider.client.endpoint(baseURL, "subtitles")
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
	if err := provider.requestJSON(ctx, "search", http.MethodGet, endpoint.String(), nil, token, &response); err != nil {
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
	endpoint, err := provider.client.endpoint(baseURL, "download")
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{"file_id": fileID, "sub_format": "srt"})
	var response openSubtitlesDownload
	if err := provider.requestJSON(ctx, "download", http.MethodPost, endpoint.String(), body, token, &response); err != nil || len(response.Link) == 0 || len(response.Link) > 2048 {
		return nil, errors.New("OpenSubtitles download failed")
	}
	return provider.client.file(ctx, providerCall{route: "file", method: http.MethodGet, endpoint: response.Link})
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
	endpoint, err := provider.client.endpoint(provider.config.URL, "login")
	if err != nil {
		return "", "", err
	}
	body, _ := json.Marshal(map[string]string{"username": provider.config.Username, "password": provider.config.Password})
	var login openSubtitlesLogin
	if err := provider.requestJSON(ctx, "login", http.MethodPost, endpoint.String(), body, "", &login); err != nil {
		return "", "", err
	}
	if login.Status != http.StatusOK || !validSubtitleProviderCredential(login.Token) {
		provider.client.health.observe("OpenSubtitles", nil, errors.New("invalid login response"))
		return "", "", errors.New("OpenSubtitles login failed")
	}
	if login.BaseURL != "" && login.BaseURL != "api.opensubtitles.com" && login.BaseURL != "vip-api.opensubtitles.com" {
		provider.client.health.observe("OpenSubtitles", nil, errors.New("invalid login host"))
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

func (provider *openSubtitlesProvider) requestJSON(ctx context.Context, route, method, endpoint string, body []byte, token string, target any) error {
	header := http.Header{}
	header.Set("Api-Key", provider.config.APIKey)
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	expired := func() bool { return token != "" && provider.forgetToken(token) }
	return provider.client.json(ctx, providerCall{route: route, method: method, endpoint: endpoint, body: body, header: header, expired: expired}, target)
}

func (provider *openSubtitlesProvider) forgetToken(token string) bool {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.token != token {
		return false
	}
	provider.token, provider.tokenExpires = "", time.Time{}
	return true
}
