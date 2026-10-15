package server

import (
	"bufio"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var removedRouteWildcard = regexp.MustCompile(`\{[^}]+\}`)

func removedRoutePatterns(t *testing.T) []string {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", "removed-routes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			patterns = append(patterns, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(patterns) == 0 {
		t.Fatal("removed route list is empty")
	}
	return patterns
}

func removedRoutes(t *testing.T) [][2]string {
	t.Helper()
	var routes [][2]string
	for _, line := range removedRoutePatterns(t) {
		method, pattern, found := strings.Cut(line, " ")
		if !found || !strings.HasPrefix(pattern, "/") {
			t.Fatalf("invalid removed route %q", line)
		}
		path := removedRouteWildcard.ReplaceAllStringFunc(pattern, func(wildcard string) string {
			if strings.HasSuffix(wildcard, "...}") {
				return "removed-probe/x"
			}
			return "removed-probe"
		})
		routes = append(routes, [2]string{method, path})
	}
	return routes
}

func TestRemovedPlayerRoutesAreNotRegistered(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.2016.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := New(Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	for _, route := range removedRoutes(t) {
		t.Run(route[0]+" "+route[1], func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), route[0], route[1], strings.NewReader("{}"))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status=%d, want 404", response.Code)
			}
		})
	}
}

func TestRemovedPlayerPackagesAreNotImported(t *testing.T) {
	removed := map[string]bool{
		"github.com/MikeO7/kinosail/packages/dlna":           true,
		"github.com/MikeO7/kinosail/packages/jellyfincompat": true,
		"github.com/MikeO7/kinosail/packages/mediashares":    true,
		"github.com/MikeO7/kinosail/packages/quickconnect":   true,
		"github.com/MikeO7/kinosail/packages/watchrooms":     true,
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if removed[path] {
				t.Errorf("%s imports removed Player package %s", name, path)
			}
		}
	}
}
