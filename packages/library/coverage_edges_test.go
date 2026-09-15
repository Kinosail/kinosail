package library

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestScanRemainingErrorAndCancellationEdges(t *testing.T) { //nolint:cyclop // One white-box matrix covers independent filesystem and cancellation failures.
	t.Parallel()
	want := errors.New("failed")
	catalog := &scanCatalog{files: make(map[string]struct{}), media: make(map[string]struct{}), subtitles: make(map[string][]string)}
	entry := failingDirEntry{infoErr: want}
	if _, err := scanItemWithRelative("root", "library", "path", entry, "video", catalog, func(string, string) (string, error) { return "", want }); !errors.Is(err, want) {
		t.Fatalf("relative path error = %v", err)
	}
	if _, err := scanItem("root", "library", "path", entry, "video", catalog); !errors.Is(err, want) {
		t.Fatalf("entry info error = %v", err)
	}

	metadataPath := filepath.Join(t.TempDir(), "movie.nfo")
	if err := os.WriteFile(metadataPath, []byte("<movie/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if metadata := readMetadataWith(metadataPath, func(string) (*os.File, error) { return nil, want }); !reflect.DeepEqual(metadata, localMetadata{}) {
		t.Fatalf("failed metadata read = %#v", metadata)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	files := []scanFile{{path: "movie", entry: entry, kind: "video"}}
	if _, err := scanFiles(ctx, "root", "library", catalog, files); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled scan = %v", err)
	}
	root := t.TempDir()
	if _, err := scanContextWith(t.Context(), root, "library", func(context.Context, string, string, *scanCatalog, []scanFile) ([]scanResult, error) {
		return nil, want
	}); !errors.Is(err, want) {
		t.Fatalf("scan propagation = %v", err)
	}

	jobs := make(chan int, 1)
	jobs <- 0
	close(jobs)
	results := make([]scanResult, 1)
	var wait sync.WaitGroup
	wait.Add(1)
	scanWorker(ctx, "root", "library", catalog, files, results, jobs, &wait)
	if results[0].included {
		t.Fatal("canceled worker included an item")
	}
	if _, err := scannedItems([]scanResult{{err: want}}); !errors.Is(err, want) {
		t.Fatalf("scan result error = %v", err)
	}
}

func TestOrganizationRemainingOrderingAndArtworkEdges(t *testing.T) {
	t.Parallel()
	shows := organizedShows(map[string]*Show{
		"z": {Title: "Zulu"},
		"a": {Title: "Alpha", Episodes: []Item{{Season: 1, Episode: 2, SeasonArtwork: "art"}, {Season: 1, Episode: 1, SeasonArtwork: ""}}},
	})
	if len(shows) != 2 || shows[0].Title != "Alpha" || len(shows[0].Seasons) != 1 || shows[0].Seasons[0].Artwork != "art" {
		t.Fatalf("organized shows = %#v", shows)
	}
	_, albums := OrganizeMusic([]Item{
		{Kind: "audio", Album: "Zulu", Artist: "Zulu", Disc: 1, Track: 2},
		{Kind: "audio", Album: "Alpha", Artist: "Alpha", Disc: 1, Track: 1},
	})
	if len(albums) != 2 || albums[0].Title != "Alpha" {
		t.Fatalf("organized albums = %#v", albums)
	}
}

type failingDirEntry struct{ infoErr error }

func (failingDirEntry) Name() string                     { return "movie.mkv" }
func (failingDirEntry) IsDir() bool                      { return false }
func (failingDirEntry) Type() fs.FileMode                { return 0 }
func (entry failingDirEntry) Info() (fs.FileInfo, error) { return nil, entry.infoErr }
