package metadata

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type providerBody struct {
	io.Reader
	closed bool
}

type cancelOnRead struct {
	io.Reader
	cancel context.CancelFunc
}

func (reader cancelOnRead) Read(data []byte) (int, error) {
	count, err := reader.Reader.Read(data)
	reader.cancel()
	return count, err
}

type providerJSONCase struct {
	name, body string
	status     int
	wantError  bool
}

type providerImageCase struct {
	name, contentType string
	status, size      int
	readFailure       bool
	wantError         bool
}

func (body *providerBody) Close() error {
	body.closed = true
	return nil
}

func TestGetProviderJSON(t *testing.T) {
	t.Parallel()
	for _, test := range []providerJSONCase{
		{"valid", `{"name":"Home","provider_extension":true}`, http.StatusOK, false},
		{"exact limit", `{"name":"Home"}` + strings.Repeat(" ", (2<<20)-15), http.StatusOK, false},
		{"oversized", `{"name":"Home"}` + strings.Repeat(" ", 2<<20), http.StatusOK, true},
		{"missing", "", http.StatusOK, true},
		{"malformed", "{", http.StatusOK, true},
		{"trailing document", `{"name":"Home"}{}`, http.StatusOK, true},
		{"rejected", `{"name":"Home"}`, http.StatusForbidden, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertProviderJSONCase(t, test)
		})
	}
}

func assertProviderJSONCase(t *testing.T, test providerJSONCase) {
	t.Helper()
	body := &providerBody{Reader: strings.NewReader(test.body)}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != "https://provider.example/movie/1" || request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected provider request: %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: test.status, Body: body}, nil
	})}
	var target struct{ Name string }
	err := GetProviderJSON(t.Context(), client, "https://provider.example/movie/1", "test-token", &target)
	if (err != nil) != test.wantError || !body.closed {
		t.Fatalf("error=%v closed=%v", err, body.closed)
	}
	if !test.wantError && target.Name != "Home" {
		t.Fatalf("decoded name=%q", target.Name)
	}
	if test.status != http.StatusOK && target.Name != "" {
		t.Fatal("rejected response changed target")
	}
}

func TestDownloadProviderImage(t *testing.T) {
	t.Parallel()
	for _, test := range []providerImageCase{
		{"valid", "image/jpeg", http.StatusOK, 10, false, false},
		{"exact limit", "image/png", http.StatusOK, 8 << 20, false, false},
		{"oversized", "image/jpeg", http.StatusOK, (8 << 20) + 1, false, true},
		{"missing type", "", http.StatusOK, 10, false, true},
		{"wrong type", "text/html", http.StatusOK, 10, false, true},
		{"rejected", "image/jpeg", http.StatusNotFound, 10, false, true},
		{"read failure", "image/jpeg", http.StatusOK, 0, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertProviderImageCase(t, test)
		})
	}
}

func TestDownloadProviderImageCanceledAfterReadDoesNotWriteCache(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	target := filepath.Join(t.TempDir(), "poster.jpg")
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/jpeg"}}, Body: io.NopCloser(cancelOnRead{Reader: strings.NewReader("image"), cancel: cancel})}, nil
	})}
	if err := DownloadProviderImage(ctx, client, "https://provider.example", "poster.jpg", target); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled download error = %v", err)
	}
	assertMissingProviderCache(t, target)
}

func assertProviderImageCase(t *testing.T, test providerImageCase) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "poster.jpg")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := &providerBody{Reader: strings.NewReader(strings.Repeat("x", test.size))}
	if test.readFailure {
		body.Reader = failingReader{}
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != "https://provider.example/images/poster.jpg" || request.Header.Get("Authorization") != "" {
			t.Fatalf("unexpected image request: %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: test.status, Header: http.Header{"Content-Type": {test.contentType}}, Body: body}, nil
	})}
	err := DownloadProviderImage(t.Context(), client, "https://provider.example/images///", "///poster.jpg", target)
	if (err != nil) != test.wantError || !body.closed {
		t.Fatalf("error=%v closed=%v", err, body.closed)
	}
	want := strings.Repeat("x", test.size)
	if test.wantError {
		want = "original"
	}
	assertProviderCache(t, target, want)
}

func assertProviderCache(t *testing.T, target, want string) {
	t.Helper()
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatal("cached image differs from expected preserved or downloaded data")
	}
}

func TestProviderHTTPFailures(t *testing.T) {
	t.Parallel()
	t.Run("invalid URL", func(t *testing.T) {
		result := providerHTTPFailure(t, "://invalid")
		if result.jsonErr == nil || result.imageErr == nil || result.target != "" || result.calls != 0 {
			t.Fatalf("invalid URL errors=%v/%v target=%q calls=%d", result.jsonErr, result.imageErr, result.target, result.calls)
		}
		assertMissingProviderCache(t, result.path)
	})
	t.Run("transport", func(t *testing.T) {
		result := providerHTTPFailure(t, "https://provider.example")
		if result.calls != 2 || !errors.Is(result.jsonErr, result.want) || !errors.Is(result.imageErr, result.want) || result.target != "" {
			t.Fatalf("transport errors=%v/%v target=%q calls=%d", result.jsonErr, result.imageErr, result.target, result.calls)
		}
		assertMissingProviderCache(t, result.path)
	})
}

type providerFailure struct {
	calls             int
	jsonErr, imageErr error
	want              error
	target, path      string
}

func providerHTTPFailure(t *testing.T, endpoint string) providerFailure {
	t.Helper()
	want := errors.New("transport failed")
	result := providerFailure{want: want}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		result.calls++
		return nil, want
	})}
	var target struct{ Name string }
	result.jsonErr = GetProviderJSON(t.Context(), client, endpoint, "", &target)
	result.path = filepath.Join(t.TempDir(), "poster.jpg")
	result.imageErr = DownloadProviderImage(t.Context(), client, endpoint, "poster.jpg", result.path)
	result.target = target.Name
	return result
}

func assertMissingProviderCache(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed request created cache: %v", err)
	}
}

func TestProviderHTTPReadAndWriteFailures(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(failingReader{})}, nil
	})}
	var target struct{ Name string }
	if err := GetProviderJSON(t.Context(), client, "https://provider.example", "", &target); err == nil {
		t.Fatal("JSON read failure succeeded")
	}
	client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		response := providerResponse(http.StatusOK, "image")
		response.Header.Set("Content-Type", "image/jpeg")
		return response, nil
	})
	if err := DownloadProviderImage(t.Context(), client, "https://provider.example", "poster.jpg", t.TempDir()); err == nil {
		t.Fatal("cache directory was replaced with an image")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, request.Context().Err()
	})
	if err := GetProviderJSON(ctx, client, "https://provider.example", "", &target); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request=%v", err)
	}
}
