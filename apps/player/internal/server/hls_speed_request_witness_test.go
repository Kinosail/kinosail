package server_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHLSSpeedRequestWitnessNormalizesSessionQuery(t *testing.T) {
	cache, segment := speedRequestWitnessCache(t)
	for _, query := range []string{"", "playbackSession=hls-speed-fixture", "playbackSession=" + strings.Repeat("a", 8), "playbackSession=" + strings.Repeat("Z", 64)} {
		request := &http.Request{URL: &url.URL{Path: "/hls/private-item/p/private-recipe/180p/segment-00013.m4s", RawQuery: query}}
		original := request.URL.String()
		target := speedFailureRequestTarget(request)
		if target != "180p/segment-00013.m4s" {
			t.Fatal("valid request did not project its fragment path")
		}
		assertSpeedRequestCacheFacts(t, cache, target)
		if request.URL.String() != original {
			t.Fatal("request changed during observation")
		}
	}
	data, err := os.ReadFile(segment)
	if err != nil || string(data) != "synthetic-media" {
		t.Fatal("cache observation modified the segment")
	}
}

func speedRequestWitnessCache(t *testing.T) (string, string) {
	t.Helper()
	cache := t.TempDir()
	segment := filepath.Join(cache, "private-recipe", "180p", "segment-00013.m4s")
	if err := os.MkdirAll(filepath.Dir(segment), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{segment: "synthetic-media", filepath.Join(cache, "private-recipe", ".source"): "synthetic-binding"} {
		if err := os.WriteFile(name, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return cache, segment
}

func assertSpeedRequestCacheFacts(t *testing.T, cache, target string) {
	t.Helper()
	facts := speedFailureCacheFacts(cache, target)
	if len(facts) != 1 || facts[0].Files["segment"].State != "regular" || facts[0].Files["segment"].Bytes != 15 {
		t.Fatalf("query-bearing fragment lost its cache witness: %+v", facts)
	}
	data, _ := json.Marshal(facts)
	if strings.Contains(string(data), "private-") || strings.Contains(string(data), "playbackSession") {
		t.Fatal("cache observation leaked private query or path")
	}
}

func TestHLSSpeedRequestWitnessRejectsAmbiguousInputs(t *testing.T) {
	path := "/hls/private-item/p/private-recipe/180p/segment-00013.m4s"
	for _, query := range []string{
		"playbackSession", "playbackSession=", "playbackSession=short", "playbackSession=" + strings.Repeat("a", 65),
		"playbackSession=valid-session&playbackSession=other-session", "playbackSession=valid-session&%70laybackSession=other-session",
		"unknown=valid-session", "playbackSession=valid-session&unknown=x", "playbackSession=valid-session;unknown=x",
		"playbackSession=%zz", "playbackSession=valid+session", "playbackSession=%00bad-session", "playbackSession=" + strings.Repeat("a", 2048),
		"&", "playbackSession=valid-session&&",
	} {
		request := &http.Request{URL: &url.URL{Path: path, RawQuery: query}}
		original := request.URL.String()
		if speedFailureRequestTarget(request) != "" || request.URL.String() != original {
			t.Fatal("malformed query admitted or request mutated")
		}
	}
	for _, value := range []*url.URL{
		nil, {}, {Path: "180p/segment-00013.m4s"}, {Path: path, Scheme: "https", Host: "private.invalid"},
		{Path: path, Fragment: "private"}, {Path: path, ForceQuery: true},
		{Path: path, RawPath: strings.Replace(path, "180p", "%31%38%30p", 1)},
		{Path: strings.Replace(path, "/p/", "/../", 1)}, {Path: strings.Replace(path, "/p/", "//", 1)},
		{Path: "/" + strings.Repeat("x", 2048) + "/180p/segment-00013.m4s"},
		{Path: strings.Replace(path, "180p", "0180p", 1)}, {Path: strings.Replace(path, "00013", "0001X", 1)},
	} {
		if speedFailureRequestTarget(&http.Request{URL: value}) != "" {
			t.Fatal("unsafe request path admitted")
		}
	}
	if speedFailureRequestTarget(nil) != "" {
		t.Fatal("missing request admitted")
	}
}
