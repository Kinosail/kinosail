package metadata

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestTMDBClientFailureBoundaries(t *testing.T) { //nolint:cyclop // Request construction and transport failures are separate provider boundaries.
	t.Parallel()
	if got := (*TMDBClient)(nil).Enrich(t.Context(), []library.Item{{Title: "Local"}}); got[0].Title != "Local" {
		t.Fatalf("nil client enrichment = %#v", got)
	}
	failingHTTP := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}
	client := &TMDBClient{token: "token", baseURL: "https://example.com", imageURL: "https://example.com", cacheDir: t.TempDir(), http: failingHTTP}
	items := []library.Item{{ID: "movie", Kind: "video", Title: "Movie"}}
	if got := client.Enrich(t.Context(), items); got[0].Title != "Movie" {
		t.Fatalf("failed enrichment changed item: %#v", got)
	}
	if err := client.get(t.Context(), "/movie/1", &tmdbMovie{}); err == nil {
		t.Fatal("TMDB transport failure was ignored")
	}
	client.baseURL = "http://[::1"
	if err := client.get(t.Context(), "/movie/1", &tmdbMovie{}); err == nil {
		t.Fatal("invalid TMDB request URL was accepted")
	}
	client.imageURL = "http://[::1"
	if _, err := client.download(t.Context(), "/poster.jpg", filepath.Join(t.TempDir(), "poster")); err == nil {
		t.Fatal("invalid image request URL was accepted")
	}
	client.imageURL = "https://example.com"
	if _, err := client.download(t.Context(), "/poster.jpg", filepath.Join(t.TempDir(), "poster")); err == nil {
		t.Fatal("image transport failure was ignored")
	}
	if _, err := client.download(t.Context(), "/../poster.jpg", filepath.Join(t.TempDir(), "poster")); err == nil {
		t.Fatal("invalid image path was accepted")
	}
}

func TestTMDBDownloadUsesDefaultExtensionAndCastLimit(t *testing.T) {
	t.Parallel()
	client := &TMDBClient{imageURL: "https://example.com", http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		response := providerResponse(http.StatusOK, "image")
		response.Header.Set("Content-Type", "image/jpeg")
		return response, nil
	})}}
	destination := filepath.Join(t.TempDir(), "poster")
	got, err := client.download(t.Context(), "/poster", destination)
	if err != nil || got != destination+".jpg" {
		t.Fatalf("download = %q, %v", got, err)
	}
	cast := make([]TMDBCastMember, 16)
	for index := range cast {
		cast[index].Name = "Actor"
	}
	if people := client.downloadCast(t.Context(), cast, t.TempDir()); len(people) != 15 {
		t.Fatalf("bounded cast size = %d", len(people))
	}
}

func TestTMDBImageCacheReturnsFilesystemAndReaderFailures(t *testing.T) { //nolint:cyclop // Atomic image caching must surface every reachable local failure.
	t.Parallel()
	root := t.TempDir()
	parentFile := filepath.Join(root, "parent")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cacheImage(strings.NewReader("image"), filepath.Join(parentFile, "poster.jpg")); err == nil {
		t.Fatal("cache directory failure was ignored")
	}
	readOnly := filepath.Join(root, "read-only")
	if err := os.Mkdir(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) }) //nolint:gosec // Cleanup restores owner-only directory traversal in the test sandbox.
	if _, err := cacheImage(strings.NewReader("image"), filepath.Join(readOnly, "poster.jpg")); err == nil {
		t.Fatal("cache temporary-file failure was ignored")
	}
	if _, err := cacheImage(failingReader{}, filepath.Join(root, "copy", "poster.jpg")); err == nil {
		t.Fatal("cache copy failure was ignored")
	}
	chmodDir := filepath.Join(root, "chmod")
	if _, err := cacheImage(&removeTemporaryReader{directory: chmodDir}, filepath.Join(chmodDir, "poster.jpg")); err == nil {
		t.Fatal("cache chmod failure was ignored")
	}
	renameTarget := filepath.Join(root, "target")
	if err := os.Mkdir(renameTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := cacheImage(strings.NewReader("image"), renameTarget); err == nil {
		t.Fatal("cache rename failure was ignored")
	}
}

func TestTMDBCacheRejectsMalformedAndInaccessibleEntries(t *testing.T) { //nolint:cyclop // Cache directory, file, JSON, and cast validation fail independently.
	t.Parallel()
	if (*TMDBClient)(nil).Active() || (&TMDBClient{}).Active() || !(&TMDBClient{token: "token"}).Active() {
		t.Fatal("TMDB activity without a cache was misreported")
	}
	empty := t.TempDir()
	if HasTMDBCache(empty) {
		t.Fatal("empty TMDB cache was reported populated")
	}
	if err := os.WriteFile(filepath.Join(empty, "plain-file"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if HasTMDBCache(empty) {
		t.Fatal("non-directory cache entry was accepted")
	}
	client := &TMDBClient{cacheDir: empty}
	metadataDir := filepath.Join(empty, "movie")
	if err := os.Mkdir(metadataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(metadataDir, "metadata.json")
	if err := os.WriteFile(metadataPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if metadata, fresh := client.Load("movie"); fresh || metadata.Title != "" {
		t.Fatalf("malformed cache loaded: %#v, %v", metadata, fresh)
	}
	if err := os.Chmod(metadataPath, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(metadataPath, 0o600) })
	if metadata, fresh := client.Load("movie"); fresh || metadata.Title != "" {
		t.Fatalf("inaccessible cache loaded: %#v, %v", metadata, fresh)
	}
	if validCachedTMDBCast([]library.Person{{}}, metadataDir) {
		t.Fatal("invalid cached cast member accepted")
	}
}

func TestTMDBFetchRejectsIMDbAndCastProviderFailures(t *testing.T) {
	t.Parallel()
	responses := []string{
		`{"movie_results":[]}`,
		`{"results":[{"id":1}]}`,
		`{"title":"Movie","credits":{"cast":[{"name":"Actor","profile_path":"/../bad"}]}}`,
	}
	client := &TMDBClient{token: "token", baseURL: "https://example.com", imageURL: "https://example.com", cacheDir: t.TempDir()}
	client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		response := providerResponse(http.StatusOK, responses[0])
		responses = responses[1:]
		return response, nil
	})}
	if _, err := client.fetch(t.Context(), library.Item{ID: "movie", Title: "Movie", ProviderIDs: map[string]string{"imdb": "tt1234567"}}); err == nil {
		t.Fatal("invalid TMDB cast was accepted")
	}
	if ValidTMDBCast(make([]TMDBCastMember, 1001)) {
		t.Fatal("oversized TMDB cast was accepted")
	}
}

func TestTMDBFetchReturnsIMDbAndMovieDetailFailures(t *testing.T) {
	t.Parallel()
	for name, item := range map[string]library.Item{
		"IMDb":          {ID: "movie", Title: "Movie", ProviderIDs: map[string]string{"imdb": "tt1234567"}},
		"movie details": {ID: "movie", Title: "Movie"},
	} {
		t.Run(name, func(t *testing.T) {
			client := &TMDBClient{token: "token", baseURL: "https://example.com", imageURL: "https://example.com", cacheDir: t.TempDir()}
			client.http = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if name == "IMDb" || strings.HasPrefix(request.URL.Path, "/movie/") {
					return nil, errors.New("offline")
				}
				return providerResponse(http.StatusOK, `{"results":[{"id":1}]}`), nil
			})}
			if _, err := client.fetch(t.Context(), item); err == nil {
				t.Fatal("provider failure was ignored")
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type removeTemporaryReader struct {
	directory string
	done      bool
}

func (reader *removeTemporaryReader) Read(buffer []byte) (int, error) {
	if reader.done {
		return 0, io.EOF
	}
	reader.done = true
	entries, _ := os.ReadDir(reader.directory)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmdb-") {
			_ = os.Remove(filepath.Join(reader.directory, entry.Name()))
		}
	}
	copy(buffer, "image")
	return len("image"), nil
}
