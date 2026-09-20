package updatecontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

const (
	githubReleasesAPIURL = "https://api.github.com/repos/Kinosail/kinosail/releases?per_page=100"
	GitHubReleasesURL    = "https://github.com/Kinosail/kinosail/releases"
	githubAPIVersion     = "2026-03-10"
	maxReleaseResponse   = 1 << 20
	maxReleaseCount      = 100
)

var errNoPublicRelease = errors.New("GitHub has no public release")

// HTTPClient sends the fixed GitHub release request.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type githubReleaseSource struct {
	client    HTTPClient
	tagPrefix string
}

type githubRelease struct {
	URL             json.RawMessage `json:"url"`
	AssetsURL       json.RawMessage `json:"assets_url"`
	UploadURL       json.RawMessage `json:"upload_url"`
	HTMLURL         json.RawMessage `json:"html_url"`
	ID              json.RawMessage `json:"id"`
	Author          json.RawMessage `json:"author"`
	NodeID          json.RawMessage `json:"node_id"`
	Tag             *string         `json:"tag_name"`
	TargetCommitish json.RawMessage `json:"target_commitish"`
	Name            json.RawMessage `json:"name"`
	Draft           *bool           `json:"draft"`
	Immutable       json.RawMessage `json:"immutable"`
	Prerelease      *bool           `json:"prerelease"`
	CreatedAt       json.RawMessage `json:"created_at"`
	UpdatedAt       json.RawMessage `json:"updated_at"`
	PublishedAt     json.RawMessage `json:"published_at"`
	Assets          json.RawMessage `json:"assets"`
	TarballURL      json.RawMessage `json:"tarball_url"`
	ZipballURL      json.RawMessage `json:"zipball_url"`
	Body            json.RawMessage `json:"body"`
	BodyHTML        json.RawMessage `json:"body_html"`
	BodyText        json.RawMessage `json:"body_text"`
	MentionsCount   json.RawMessage `json:"mentions_count"`
	DiscussionURL   json.RawMessage `json:"discussion_url"`
	Reactions       json.RawMessage `json:"reactions"`
}

// NewGitHubChecker creates a checker for the app release identity in policy.
func NewGitHubChecker(client HTTPClient, policy Policy, config CheckerConfig) (*Checker, error) {
	if client == nil || validatePolicy(policy) != nil || config.Source != nil {
		return nil, errors.New("invalid GitHub update checker configuration")
	}
	source := githubReleaseSource{client: client, tagPrefix: policy.tagPrefix + "-"}
	config.Source = ReleaseSourceFunc(source.Latest)
	return NewChecker(config)
}

func (source githubReleaseSource) Latest(ctx context.Context, etag string) (string, string, bool, error) {
	if ctx == nil {
		return "", "", false, errors.New("invalid release context")
	}
	request := githubReleaseRequest(ctx, etag)
	response, err := source.client.Do(request)
	if err != nil {
		return "", "", false, errors.New("GitHub could not be reached")
	}
	defer response.Body.Close()
	responseETag := boundedETag(response.Header.Get("ETag"))
	if unchanged, handled, err := githubReleaseStatus(response.StatusCode); handled {
		return "", responseETag, unchanged, err
	}
	releases, err := decodeGitHubReleases(response)
	if err != nil {
		return "", "", false, err
	}
	version, err := publicGitHubRelease(releases, source.tagPrefix)
	return version, responseETag, false, err
}

func githubReleaseRequest(ctx context.Context, etag string) *http.Request {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesAPIURL, nil)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	request.Header.Set("User-Agent", "Kinosail-update-check")
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	return request
}

func githubReleaseStatus(status int) (bool, bool, error) {
	switch status {
	case http.StatusNotModified:
		return true, true, nil
	case http.StatusNotFound:
		return false, true, errNoPublicRelease
	case http.StatusOK:
		return false, false, nil
	default:
		return false, true, errors.New("GitHub did not return a release")
	}
}

func decodeGitHubReleases(response *http.Response) ([]githubRelease, error) {
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, errors.New("GitHub returned an invalid response")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxReleaseResponse+1))
	if err != nil || len(body) > maxReleaseResponse {
		return nil, errors.New("GitHub returned an invalid response")
	}
	var releases []githubRelease
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&releases); err != nil || releases == nil || len(releases) > maxReleaseCount {
		return nil, errors.New("GitHub returned an invalid response")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("GitHub returned an invalid response")
	}
	return releases, nil
}

func publicGitHubRelease(releases []githubRelease, tagPrefix string) (string, error) {
	for _, release := range releases {
		if release.Tag == nil || release.Draft == nil || release.Prerelease == nil {
			return "", errors.New("GitHub returned an invalid release")
		}
		if *release.Draft || *release.Prerelease || !strings.HasPrefix(*release.Tag, tagPrefix) {
			continue
		}
		version := strings.TrimPrefix(*release.Tag, tagPrefix)
		if !ValidReleaseVersion(version) {
			return "", errors.New("GitHub returned an invalid release version")
		}
		return version, nil
	}
	return "", errNoPublicRelease
}

func boundedETag(value string) string {
	if len(value) > 512 || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}
