//go:build linux || darwin

package server_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest"
)

// Unsynchronized overlap exercises registered HTTP; it does not prove the
// captured c6 schedule. The isolated inspection test owns that exact boundary.
func TestHLSHotManifestHTTPOverlappingAtomicPublisher(t *testing.T) {
	for _, offset := range []string{"", "-o1400"} {
		for _, name := range []string{"index.m3u8", "360p/index.m3u8"} {
			t.Run(offset+"/"+name, func(t *testing.T) { overlapHotHTTPManifest(t, newCachedSegmentEvidenceFixture(t, offset), name) })
		}
	}
}

func overlapHotHTTPManifest(t *testing.T, fixture cachedSegmentEvidenceFixture, name string) {
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
	finished := overlapHotManifestPublisher(ctx, root, name, next)
	defer func() {
		cancel()
		if err := <-finished; err != nil {
			t.Error(err)
		}
	}()
	client := &http.Client{Timeout: time.Second}
	overlappingManifestHTTPRequests(t, ctx, client, fixture, peer.URL, expected)
	cancel()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	// Join once; the deferred failure cleanup still receives from the closed channel.
	after := servertest.SnapshotHLSCache(t, fixture.cache)
	delete(before, filepath.Join(fixture.directory, filepath.FromSlash(name)))
	delete(after, filepath.Join(fixture.directory, filepath.FromSlash(name)))
	encoded, err := os.ReadFile(fixture.arguments)
	if err != nil || !bytes.Equal(starts, encoded) || !reflect.DeepEqual(before, after) {
		t.Fatal("request changed non-publisher cache or encoder")
	}
}

func overlappingManifestHTTPRequests(t *testing.T, ctx context.Context, client *http.Client, fixture cachedSegmentEvidenceFixture, origin string, expected []byte) {
	t.Helper()
	for range 32 {
		request := fixture.request(ctx, origin+fixture.route+"?playbackSession=cached-session")
		request.RequestURI = ""
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1025))
		closeErr := response.Body.Close()
		assertHotManifestResponse(t, response.StatusCode, body, expected, readErr, closeErr)
	}
}

func overlapHotManifestPublisher(ctx context.Context, root *os.Root, name string, data []byte) <-chan error {
	done := make(chan error, 1)
	go func() {
		defer close(done)
		var failure error
		for ctx.Err() == nil {
			if failure = root.WriteFile(name+".next", data, 0o600); failure != nil {
				break
			}
			if failure = root.Rename(name+".next", name); failure != nil {
				break
			}
		}
		if err := root.Remove(name + ".next"); err != nil && !errors.Is(err, os.ErrNotExist) {
			failure = errors.Join(failure, err)
		}
		done <- failure
	}()
	return done
}
