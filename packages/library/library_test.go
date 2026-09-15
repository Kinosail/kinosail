package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanStopsWhenCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ScanContext(ctx, t.TempDir(), "."); !errors.Is(err, context.Canceled) {
		t.Fatalf("ScanContext() error = %v, want context canceled", err)
	}
}

func TestScanFindsVideoMusicAndPhotosWithoutSidecarArtwork(t *testing.T) { //nolint:cyclop // One fixture verifies every supported media family and artwork fallback.
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"Movie.mp4", "Movie.jpg", "Song.flac", "Song.png", "Ambient.oga", "Field.aiff", "Vacation.jpeg", "Vacation.avif", "Sketch.bmp"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	items, err := ScanContext(t.Context(), root, ".")
	if err != nil {
		t.Fatal(err)
	}
	kinds := make(map[string]string)
	for _, item := range items {
		kinds[item.Title] = item.Kind
	}
	if len(items) != 7 || kinds["Movie"] != "video" || kinds["Song"] != "audio" || kinds["Ambient"] != "audio" || kinds["Field"] != "audio" || kinds["Vacation"] != "photo" || kinds["Sketch"] != "photo" {
		t.Fatalf("scanned items = %#v", items)
	}
}

func TestScanDoesNotAdmitUnsupportedFormats(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"Camera.heic", "Novel.mobi", "Comic.cbr", "Camera.tiff"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	items, err := ScanContext(t.Context(), root, ".")
	if err != nil || len(items) != 0 {
		t.Fatalf("unapproved formats scanned = %#v, error = %v", items, err)
	}
}

func TestScanRejectsSymlinkedContentAndSidecars(t *testing.T) {
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	for _, name := range []string{"Leak.mp4", "Movie.jpg", "Movie.en.srt", "Movie.nfo"} {
		if err := os.WriteFile(filepath.Join(outside, name), []byte(`<movie><title>Leaked</title></movie>`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(outside, name), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := ScanContext(t.Context(), root, ".")
	if err != nil || len(items) != 1 || items[0].Title != "Movie" || items[0].Artwork != "" || len(items[0].Subtitles) != 0 {
		t.Fatalf("symlink scan = %#v, %v", items, err)
	}
}

func TestScanAssociatesSidecarsWithMixedCaseMediaExtensions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"Film.Mp4", "Film.jpg", "Film.en.srt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	items, err := ScanContext(t.Context(), root, ".")
	if err != nil || len(items) != 1 || items[0].Artwork == "" || len(items[0].Subtitles) != 1 {
		t.Fatalf("mixed-case scan = %#v, %v", items, err)
	}
}

func TestMediaKindCoversMainstreamFormats(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		".3g2": "video", ".3gp": "video", ".asf": "video", ".avi": "video", ".flv": "video", ".m2ts": "video", ".m4v": "video", ".mkv": "video", ".mov": "video", ".mp4": "video", ".mpeg": "video", ".mpg": "video", ".mts": "video", ".ogv": "video", ".ts": "video", ".vob": "video", ".webm": "video", ".wmv": "video",
		".aac": "audio", ".aif": "audio", ".aiff": "audio", ".flac": "audio", ".m4a": "audio", ".mp3": "audio", ".oga": "audio", ".ogg": "audio", ".opus": "audio", ".wav": "audio", ".weba": "audio",
		".m4b":  "audiobook",
		".avif": "photo", ".bmp": "photo", ".gif": "photo", ".jpeg": "photo", ".jpg": "photo", ".png": "photo", ".webp": "photo",
		".cb7": "book", ".cbt": "book", ".cbz": "book", ".epub": "book", ".pdf": "book",
	}
	for extension, kind := range want {
		if got := mediaKind(strings.ToUpper(extension)); got != kind {
			t.Errorf("mediaKind(%q) = %q, want %q", extension, got, kind)
		}
	}
	for _, extension := range []string{".aax", ".azw3", ".cbr", ".exe", ".heic", ".mobi", ".svg"} {
		if got := mediaKind(extension); got != "" {
			t.Errorf("mediaKind(%q) = %q, want unsupported", extension, got)
		}
	}
}

func TestScanClassifiesMP4InsideAudiobookLibraryAsAudiobook(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Chapter.mp4"), []byte("audio-only mp4"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := ScanContext(t.Context(), root, "Audiobooks")
	if err != nil || len(items) != 1 || items[0].Kind != "audiobook" {
		t.Fatalf("audiobook MP4 scan = %#v, %v", items, err)
	}
}

func TestSubtitleIndexStopsAtOrphanStem(t *testing.T) {
	t.Parallel()
	catalog := &scanCatalog{media: make(map[string]struct{}), subtitles: make(map[string][]string), subtitleFiles: []string{"/library/Orphan.en.srt"}}
	done := make(chan struct{})
	go func() {
		catalog.indexSubtitles()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("orphan subtitle indexing did not terminate")
	}
	if len(catalog.subtitles) != 0 {
		t.Fatalf("orphan subtitles = %#v", catalog.subtitles)
	}
}

func TestScanReadsStableProviderIdentifiersFromNFO(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Arrival.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Arrival.nfo"), []byte(`<movie><uniqueid type="tmdb">329865</uniqueid><uniqueid type="imdb">tt2543164</uniqueid><uniqueid type="unsupported">ignored</uniqueid></movie>`), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := ScanContext(t.Context(), root, ".")
	if err != nil || len(items) != 1 || items[0].ProviderIDs["tmdb"] != "329865" || items[0].ProviderIDs["imdb"] != "tt2543164" || items[0].ProviderIDs["unsupported"] != "" {
		t.Fatalf("scanned provider identifiers = %#v, error = %v", items, err)
	}
}

func TestScanSeparatesMovieTitleAndYearFromReleaseMetadata(t *testing.T) { //nolint:cyclop // One scan verifies all release-name parsing cases.
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{
		"1917.mp4",
		"28 Years Later The Bone Temple (2026) {imdb tt32141377} [Bluray 1080p][EAC3 5 1][x264] hallowed.mkv",
		"Harry Potter and the Deathly Hallows Part 2 (2011) {imdb-tt1201607} [Bluray-1080p][AC3 5.1][x264]-BHDStudio.mp4",
		"'Twas the Night Before Christmas (1974) {imdb tt0208654} [Bluray 1080p] [DTS 1 0][x264] SADPANDA.mkv",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	items, err := ScanContext(t.Context(), root, ".")
	if err != nil || len(items) != 4 {
		t.Fatalf("scanned release filename = %#v, error = %v", items, err)
	}
	byTitle := make(map[string]Item, len(items))
	for _, item := range items {
		byTitle[item.Title] = item
	}
	if byTitle["1917"].Year != "" || byTitle["28 Years Later The Bone Temple"].Year != "2026" || byTitle["Harry Potter and the Deathly Hallows Part 2"].Year != "2011" || byTitle["'Twas the Night Before Christmas"].Year != "1974" || byTitle["'Twas the Night Before Christmas"].ProviderIDs["imdb"] != "tt0208654" {
		t.Fatalf("scanned release filename = %#v", items)
	}
}

func TestScanSeparatesShowTitleYearAndProviderIDFromFolder(t *testing.T) { //nolint:cyclop,gocognit // One table-driven scan covers the full parsed show identity contract.
	t.Parallel()
	root := t.TempDir()
	for _, path := range []string{
		"Tumble Leaf (2014) {imdb-tt2948562}/Season 01/Tumble Leaf - S01E01.mkv",
		"The Golden Bachelor (AU) (2025) {imdb-}/Season 01/The Golden Bachelor - S01E01.mkv",
		"Unknown ID (2025) {imdb-not-an-id}/Season 01/Unknown ID - S01E01.mkv",
		"Oversized ID (2025) {imdb-tt1111111111111111111111111111111111111111}/Season 01/Oversized ID - S01E01.mkv",
		"Conflicting IDs (2025) {imdb-tt0208654} {imdb-tt0208655}/Season 01/Conflicting IDs - S01E01.mkv",
	} {
		path = filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	items, err := ScanContext(t.Context(), root, "tv")
	if err != nil || len(items) != 5 {
		t.Fatalf("scanned shows = %#v, error = %v", items, err)
	}
	byTitle := make(map[string]Item, len(items))
	for _, item := range items {
		byTitle[item.ShowTitle] = item
	}
	if tumble := byTitle["Tumble Leaf"]; tumble.ShowYear != "2014" || tumble.ShowProviderIDs["imdb"] != "tt2948562" {
		t.Fatalf("Tumble Leaf identity = %#v", tumble)
	}
	if golden := byTitle["The Golden Bachelor (AU)"]; golden.ShowYear != "2025" || len(golden.ShowProviderIDs) != 0 {
		t.Fatalf("Golden Bachelor identity = %#v", golden)
	}
	for _, title := range []string{"Unknown ID", "Oversized ID", "Conflicting IDs"} {
		if item := byTitle[title]; item.ShowYear != "2025" || len(item.ShowProviderIDs) != 0 {
			t.Fatalf("%s identity = %#v", title, item)
		}
	}
}

func TestScanIgnoresMalformedFilenameProviderIdentifier(t *testing.T) {
	t.Parallel()
	for name, identifier := range map[string]string{
		"malformed":   "tt123",
		"unknown":     "not-an-imdb-id",
		"oversized":   "tt" + strings.Repeat("1", 40),
		"conflicting": "tt0208654} {imdb tt0208655",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			filename := "Primer (2004) {imdb " + identifier + "} [1080p].mkv"
			if err := os.WriteFile(filepath.Join(root, filename), []byte("video"), 0o600); err != nil {
				t.Fatal(err)
			}
			items, err := ScanContext(t.Context(), root, ".")
			if err != nil || len(items) != 1 || items[0].Title != "Primer" || items[0].Year != "2004" || len(items[0].ProviderIDs) != 0 {
				t.Fatalf("scanned invalid provider filename = %#v, error = %v", items, err)
			}
		})
	}
}
