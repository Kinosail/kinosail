package updatecontrol

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestGitHubReleaseCheckSendsOnlyFixedPrivacySafeMetadata(t *testing.T) { //nolint:cyclop // One contract verifies every fixed request and privacy header.
	t.Parallel()
	var captured *http.Request
	source := githubReleaseSource{client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		captured = request.Clone(request.Context())
		body := `[{"tag_name":"other-v9.9.9","draft":false,"prerelease":false},{"tag_name":"player-v1.2.3","draft":false,"prerelease":false}]`
		return githubResponse(http.StatusOK, body, `"release-1"`), nil
	})}, tagPrefix: "player-"}
	version, etag, unchanged, err := source.Latest(t.Context(), `"release-0"`)
	if err != nil || version != "v1.2.3" || etag != `"release-1"` || unchanged {
		t.Fatalf("latest = %q %q %t %v", version, etag, unchanged, err)
	}
	if captured.URL.String() != githubReleasesAPIURL || captured.Method != http.MethodGet || captured.URL.RawQuery != "per_page=100" {
		t.Fatalf("request = %s %s", captured.Method, captured.URL)
	}
	if captured.Header.Get("Accept") != "application/vnd.github+json" || captured.Header.Get("X-GitHub-Api-Version") != githubAPIVersion || captured.Header.Get("User-Agent") != "Kinosail-update-check" || captured.Header.Get("If-None-Match") != `"release-0"` {
		t.Fatalf("headers = %#v", captured.Header)
	}
	for _, name := range []string{"Authorization", "Cookie", "X-Kinosail-Version", "X-Kinosail-Server"} {
		if captured.Header.Get(name) != "" {
			t.Fatalf("private header %s was sent", name)
		}
	}
}

func TestGitHubCheckerUsesApplicationReleaseIdentity(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, tag string
		policy    Policy
	}{
		{name: "player", tag: "player-v1.2.3", policy: PlayerPolicy(1, 1)},
		{name: "subtitles", tag: "subtitles-v1.2.3", policy: SubtitlesPolicy(1, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return githubResponse(http.StatusOK, `[{"tag_name":"`+test.tag+`","draft":false,"prerelease":false}]`, ""), nil
			})}
			checker, err := NewGitHubChecker(client, test.policy, CheckerConfig{
				CurrentVersion: "v1.0.0", Automatic: func() bool { return false }, SaveAutomatic: func(bool) error { return nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			if status := checker.Check(t.Context()); status.State != "available" || status.LatestVersion != "v1.2.3" {
				t.Fatalf("status = %#v", status)
			}
		})
	}
}

func TestNewGitHubCheckerRejectsInvalidAdaptersBeforeNetworkAccess(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return githubResponse(http.StatusOK, `[]`, ""), nil
	})}
	valid := CheckerConfig{CurrentVersion: "v1.0.0", Automatic: func() bool { return false }, SaveAutomatic: func(bool) error { return nil }}
	for name, invoke := range map[string]func() (*Checker, error){
		"client": func() (*Checker, error) { return NewGitHubChecker(nil, PlayerPolicy(1, 1), valid) },
		"policy": func() (*Checker, error) { return NewGitHubChecker(client, Policy{}, valid) },
		"source": func() (*Checker, error) {
			config := valid
			config.Source = ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) { return "", "", false, nil })
			return NewGitHubChecker(client, PlayerPolicy(1, 1), config)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if checker, err := invoke(); err == nil || checker != nil {
				t.Fatalf("invalid adapters created checker %#v, err=%v", checker, err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid adapters made %d network request(s)", calls.Load())
	}
}

func TestGitHubReleaseCheckRejectsUntrustedResponsesWithoutUpdateRequest(t *testing.T) { //nolint:funlen // Security cases stay together so every rejected response shares the side-effect assertion.
	t.Parallel()
	release := `{"tag_name":"player-v1.2.3","draft":false,"prerelease":false}`
	responses := map[string]func() *http.Response{
		"wrong status": func() *http.Response { return githubResponse(http.StatusFound, "["+release+"]", "") },
		"wrong type": func() *http.Response {
			response := githubResponse(http.StatusOK, "["+release+"]", "")
			response.Header.Set("Content-Type", "text/plain")
			return response
		},
		"invalid type": func() *http.Response {
			response := githubResponse(http.StatusOK, "["+release+"]", "")
			response.Header.Set("Content-Type", "broken;")
			return response
		},
		"wrong shape":   func() *http.Response { return githubResponse(http.StatusOK, release, "") },
		"null response": func() *http.Response { return githubResponse(http.StatusOK, "null", "") },
		"missing field": func() *http.Response {
			return githubResponse(http.StatusOK, `[{"tag_name":"player-v1.2.3","draft":false}]`, "")
		},
		"unknown field": func() *http.Response {
			return githubResponse(http.StatusOK, `[{"tag_name":"player-v1.2.3","draft":false,"prerelease":false,"extra":true}]`, "")
		},
		"bad version": func() *http.Response {
			return githubResponse(http.StatusOK, `[{"tag_name":"player-latest","draft":false,"prerelease":false}]`, "")
		},
		"trailing JSON": func() *http.Response { return githubResponse(http.StatusOK, "["+release+"]{}", "") },
		"too many": func() *http.Response {
			return githubResponse(http.StatusOK, "["+strings.Repeat(release+",", maxReleaseCount)+release+"]", "")
		},
		"oversized": func() *http.Response {
			return githubResponse(http.StatusOK, `[{"tag_name":"player-v1.2.3","draft":false,"prerelease":false,"body":"`+strings.Repeat("x", maxReleaseResponse)+`"}]`, "")
		},
	}
	for name, response := range responses {
		t.Run(name, func(t *testing.T) {
			manager, err := New(nil, PlayerPolicy(1, 1))
			if err != nil {
				t.Fatal(err)
			}
			source := githubReleaseSource{client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return response(), nil })}, tagPrefix: "player-"}
			checker := mustChecker(t, CheckerConfig{
				Manager: manager, Source: source, CurrentVersion: "v1.0.0", Automatic: func() bool { return true }, SaveAutomatic: func(bool) error { return nil },
			})
			if status := checker.Check(t.Context()); status.State != "unavailable" {
				t.Fatalf("untrusted response status = %#v", status)
			}
			view, viewErr := manager.View("v1.0.0")
			if viewErr != nil || view.RequestID != "" {
				t.Fatalf("rejected response requested an update: %#v, err=%v", view, viewErr)
			}
		})
	}
}

func TestGitHubReleaseCheckHandlesCacheMissingAndTransportFailures(t *testing.T) { //nolint:cyclop // One table verifies the distinct non-release outcomes.
	t.Parallel()
	for name, source := range map[string]githubReleaseSource{
		"missing": {client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return githubResponse(http.StatusNotFound, `{"message":"Not Found"}`, ""), nil
		})}, tagPrefix: "player-"},
		"filtered": {client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			body := `[{"tag_name":"other-v9.9.9","draft":false,"prerelease":false},{"tag_name":"player-v1.1.0","draft":false,"prerelease":true},{"tag_name":"player-v1.0.0","draft":true,"prerelease":false}]`
			return githubResponse(http.StatusOK, body, `"filtered"`), nil
		})}, tagPrefix: "player-"},
		"transport": {client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("offline")
		})}, tagPrefix: "player-"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, _, err := source.Latest(t.Context(), "")
			if name == "transport" {
				if err == nil || errors.Is(err, errNoPublicRelease) {
					t.Fatalf("transport error = %v", err)
				}
			} else if !errors.Is(err, errNoPublicRelease) {
				t.Fatalf("missing release error = %v", err)
			}
		})
	}

	source := githubReleaseSource{client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return githubResponse(http.StatusNotModified, "", strings.Repeat("x", 513)), nil
	})}, tagPrefix: "player-"}
	version, etag, unchanged, err := source.Latest(t.Context(), "")
	if err != nil || version != "" || etag != "" || !unchanged {
		t.Fatalf("not modified = %q %q %t %v", version, etag, unchanged, err)
	}
	if got := boundedETag("ok\ninvalid"); got != "" {
		t.Fatalf("newline etag = %q", got)
	}
}

func githubResponse(status int, body, etag string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json; charset=utf-8"}, "Etag": {etag}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
