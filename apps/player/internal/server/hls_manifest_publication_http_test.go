package server_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest"
)

// Registered real HTTP must deliver cached bytes after each complete atomic
// publication. The descriptor test owns the exact concurrent read/rename window;
// unsynchronized stress could hit the conservative pre-open rejection boundary.
func TestHLSHotManifestHTTPAtomicPublication(t *testing.T) {
	for _, offset := range []string{"", "-o1400"} {
		for _, name := range []string{"index.m3u8", "360p/index.m3u8"} {
			t.Run(fmt.Sprintf("%s/%s", offset, name), func(t *testing.T) {
				fixture := newCachedSegmentEvidenceFixture(t, offset)
				exerciseHotManifestHTTP(t, fixture, name)
			})
		}
	}
}

func exerciseHotManifestHTTP(t *testing.T, fixture cachedSegmentEvidenceFixture, name string) {
	t.Helper()
	root, err := os.OpenRoot(fixture.directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	next := prepareHotHTTPPublication(t, root, name)
	expected, err := root.ReadFile("360p/segment-00001.m4s")
	if err != nil {
		t.Fatal(err)
	}
	before := servertest.SnapshotHLSCache(t, fixture.cache)
	starts, err := os.ReadFile(fixture.arguments)
	if err != nil {
		t.Fatal(err)
	}
	peer := httptest.NewServer(fixture.handler)
	defer peer.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	rounds, finished := startHotManifestPublisher(ctx, root, name, next)
	defer func() { cancel(); <-finished }()
	client := &http.Client{Timeout: time.Second}
	for range 32 {
		hotManifestHTTPRound(t, ctx, client, fixture, peer.URL, rounds, expected)
	}
	// All publishers are acknowledged before comparison; only their fixed manifest
	// path is excluded. Requests may not mutate unrelated cache or allocate encoding.
	after := servertest.SnapshotHLSCache(t, fixture.cache)
	delete(before, filepath.Join(fixture.directory, filepath.FromSlash(name)))
	delete(after, filepath.Join(fixture.directory, filepath.FromSlash(name)))
	encoded, err := os.ReadFile(fixture.arguments)
	if err != nil || !bytes.Equal(starts, encoded) || !reflect.DeepEqual(before, after) {
		t.Fatal("HTTP request changed non-publisher cache or encoder ownership")
	}
}

func prepareHotHTTPPublication(t *testing.T, root *os.Root, name string) []byte {
	t.Helper()
	initial, err := root.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if name != "360p/index.m3u8" {
		return initial
	}
	if err := root.WriteFile("360p/segment-00002.m4s", []byte("later fragment"), 0o600); err != nil {
		t.Fatal(err)
	}
	return []byte(strings.Replace(string(initial), "#EXT-X-ENDLIST", "#EXTINF:4,\nsegment-00002.m4s\n#EXT-X-ENDLIST", 1))
}

func hotManifestHTTPRound(t *testing.T, ctx context.Context, client *http.Client, fixture cachedSegmentEvidenceFixture, origin string, rounds chan chan error, expected []byte) {
	t.Helper()
	publication := make(chan error, 1)
	select {
	case rounds <- publication:
	case <-ctx.Done():
		t.Fatal("publisher did not start")
	}
	select {
	case err := <-publication:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("publisher did not finish")
	}
	request := fixture.request(t, origin+fixture.route+"?playbackSession=cached-session").WithContext(ctx)
	request.RequestURI = "" // Convert the registered-route request to a real client request.
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1025))
	closeErr := response.Body.Close()
	assertHotManifestResponse(t, response.StatusCode, body, expected, readErr, closeErr)
}

func startHotManifestPublisher(ctx context.Context, root *os.Root, name string, data []byte) (chan chan error, chan struct{}) {
	rounds, done := make(chan chan error), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case result := <-rounds:
				err := root.WriteFile(name+".next", data, 0o600)
				if err == nil {
					err = root.Rename(name+".next", name)
				}
				result <- err
			}
		}
	}()
	return rounds, done
}

func assertHotManifestResponse(t *testing.T, status int, body, expected []byte, readErr, closeErr error) {
	t.Helper()
	if readErr != nil || closeErr != nil {
		t.Fatal("fragment response did not complete")
	}
	if status != http.StatusOK || !bytes.Equal(body, expected) {
		t.Fatal("atomic manifest publication rejected cached fragment")
	}
}

func TestHLSHotManifestHTTPUnsafeFileHasNoRequestEffects(t *testing.T) {
	for _, damage := range []string{"empty", "oversized", "symlink", "directory", "malformed duration"} {
		t.Run(damage, func(t *testing.T) {
			fixture := newCachedSegmentEvidenceFixture(t, "")
			damageHotHTTPManifest(t, fixture, damage)
			status, events := fixture.deliverWithoutMutation(t, fixture.request(t, fixture.route))
			if status != http.StatusNotFound || len(events) != 1 {
				t.Fatal("unsafe hot index was admitted")
			}
			assertCachedSegmentEvent(t, events[0], false)
		})
	}
}

func damageHotHTTPManifest(t *testing.T, fixture cachedSegmentEvidenceFixture, damage string) {
	t.Helper()
	root, err := os.OpenRoot(fixture.directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	const name = "360p/index.m3u8"
	if err := root.Remove(name); err != nil {
		t.Fatal(err)
	}
	switch damage {
	case "symlink":
		err = root.Symlink("init.mp4", name)
	case "directory":
		err = root.Mkdir(name, 0o700)
	case "empty":
		err = root.WriteFile(name, nil, 0o600)
	case "oversized":
		err = root.WriteFile(name, bytes.Repeat([]byte("x"), (1<<20)+1), 0o600)
	case "malformed duration":
		err = root.WriteFile(name, []byte("#EXTM3U\n#EXTINF:NaN,\nsegment-00001.m4s\n"), 0o600)
	default:
		t.Fatal("unknown damage")
	}
	if err != nil {
		t.Fatal(err)
	}
}
