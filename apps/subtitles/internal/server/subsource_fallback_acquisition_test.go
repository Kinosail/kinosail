package server

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubSourceAcquisitionRecoversFromInvalidCandidate(t *testing.T) { //nolint:cyclop,funlen,gocognit // One operation matrix distinguishes candidate rejection from provider availability and checks durable effects.
	t.Parallel()
	for _, test := range []struct {
		name      string
		succeeds  bool
		downloads []string
	}{
		{"lower ranked candidate control", true, []string{"10"}},
		{"CRC mismatch", true, []string{"9", "10"}},
		{"complete truncated ZIP", true, []string{"9", "10"}},
		{"wrong episode member", true, []string{"9", "10"}},
		{"ambiguous exact members", true, []string{"9", "10"}},
		{"body above four MiB", true, []string{"9", "10"}},
		{"rate limit", false, []string{"9"}},
		{"service unavailable", false, []string{"9"}},
		{"short HTTP transfer", false, []string{"9"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := []byte("1\n00:00:01,000 --> 00:00:02,000\nUnchanged provider dialogue\n")
			good := zipSubSourceFiles(t, map[string][]byte{"Show.S01E02.srt": original})
			bad := invalidSubSourceFallbackArchive(t, test.name, good, original)
			media := filepath.Join(t.TempDir(), "Show.S01E02.mkv")
			if err := os.WriteFile(media, []byte("isolated media locator"), 0o600); err != nil {
				t.Fatal(err)
			}
			item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: media, Title: "Second", Show: "Show", ShowYear: "2024", Season: 1, Episode: 2, ShowProviderIDs: map[string]string{"imdb": "tt1234567"}}
			data := t.TempDir()
			var mu sync.Mutex
			var downloaded []string
			var beforeErr error
			var before subSourceAcquisitionEffects
			var validReached bool
			remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Header.Get("X-API-Key") != "key" {
					http.Error(writer, "missing fixture credential", http.StatusUnauthorized)
					return
				}
				switch request.URL.Path {
				case "/api/v1/movies/search":
					if request.URL.Query().Get("imdb") != "tt1234567" {
						http.Error(writer, "wrong fixture query", http.StatusBadRequest)
						return
					}
					_, _ = writer.Write([]byte(`{"success":true,"data":[{"movieId":7,"title":"Show","type":"tvseries","releaseYear":2024,"imdbId":"tt1234567","season":1,"subtitleCount":2}]}`))
				case "/api/v1/subtitles":
					candidate := func(id, rating int) map[string]any {
						files, size := 1, len(good)
						if id == 9 {
							size = len(bad)
							if test.name == "ambiguous exact members" {
								files = 2
							}
							// A one-byte body overflow must reach the HTTP reader, not fail search metadata validation.
							if test.name == "body above four MiB" {
								size = 4 << 20
							}
						}
						return map[string]any{"subtitleId": id, "movieId": 7, "language": "english", "releaseInfo": []string{"Show.S01E02.WEB-DL"}, "files": files, "size": size, "productionType": "retail", "downloads": 10, "rating": map[string]int{"good": rating, "bad": 0, "total": rating}}
					}
					candidates := []any{candidate(9, 10), candidate(10, 4)}
					if test.name == "lower ranked candidate control" {
						candidates = candidates[1:]
					}
					_ = json.NewEncoder(writer).Encode(map[string]any{"success": true, "data": candidates, "pagination": map[string]int{"page": 1, "limit": 50, "total": len(candidates), "pages": 1}})
				case "/api/v1/subtitles/9/download":
					mu.Lock()
					downloaded = append(downloaded, "9")
					mu.Unlock()
					switch test.name {
					case "rate limit":
						writer.Header().Set("Retry-After", "120")
						writer.WriteHeader(http.StatusTooManyRequests)
					case "service unavailable":
						writer.WriteHeader(http.StatusServiceUnavailable)
					case "short HTTP transfer":
						// The ZIP bytes are complete; only the HTTP body is incomplete.
						writer.Header().Set("Content-Length", strconv.Itoa(len(good)+17))
						writer.Header().Set("Connection", "close")
						_, _ = writer.Write(good)
					default:
						// The truncated-ZIP case still delivers its entire declared HTTP body.
						writer.Header().Set("Content-Type", "application/zip")
						writer.Header().Set("Content-Length", strconv.Itoa(len(bad)))
						_, _ = writer.Write(bad)
					}
				case "/api/v1/subtitles/10/download":
					effects, err := readSubSourceAcquisitionEffects(media, data)
					mu.Lock()
					downloaded = append(downloaded, "10")
					before, beforeErr, validReached = effects, err, true
					mu.Unlock()
					writer.Header().Set("Content-Type", "application/zip")
					_, _ = writer.Write(good)
				default:
					http.NotFound(writer, request)
				}
			}))
			t.Cleanup(remote.Close)
			config := SubtitleConfig{SubSource: SubSourceConfig{URL: remote.URL + "/api/v1", APIKey: "key", PersonalUse: true}}
			provider := newSubtitleProvider(config, t.TempDir(), data, sidecarTestIndex(item), nil, "")
			fetchErr := provider.fetchSidecar(t.Context(), item, "en")
			after, effectErr := readSubSourceAcquisitionEffects(media, data)
			if effectErr != nil {
				t.Fatal(effectErr)
			}
			mu.Lock()
			actual, firstEffects, firstErr, reached := slices.Clone(downloaded), before, beforeErr, validReached
			mu.Unlock()
			t.Logf("downloads=%v fetch=%v beforeReached=%v before=%+v afterCounts=sidecar:%t originals:%d records:%d history:%d", actual, fetchErr, reached, firstEffects, after.sidecarPresent, len(after.originals), after.records, after.history)
			if !slices.Equal(actual, test.downloads) {
				t.Errorf("downloads=%v want=%v", actual, test.downloads)
			}
			if !test.succeeds {
				if fetchErr == nil || reached || !after.empty() {
					t.Fatalf("availability failure changed acquisition: fetch=%v reached=%v effects=%+v", fetchErr, reached, after)
				}
				return
			}
			if !reached {
				t.Error("valid fallback candidate was not downloaded")
			} else if firstErr != nil || !firstEffects.empty() {
				t.Errorf("partial acquisition before valid candidate: error=%v effects=%+v", firstErr, firstEffects)
			}
			if fetchErr != nil || !after.sidecarPresent || !bytes.Equal(after.sidecar, original) || after.extraMediaFiles != 1 || after.records != 1 || after.history != 1 || len(after.originals) != 1 || !bytes.Equal(after.originals[0], original) {
				t.Fatalf("expected one byte-preserved acquisition: fetch=%v effects=%+v", fetchErr, after)
			}
		})
	}
}

func invalidSubSourceFallbackArchive(t *testing.T, mode string, good, original []byte) []byte {
	t.Helper()
	switch mode {
	case "CRC mismatch":
		return corruptSubSourceFallbackArchive(t, good)
	case "complete truncated ZIP":
		bad := bytes.Clone(good[:len(good)-22])
		if _, err := zip.NewReader(bytes.NewReader(bad), int64(len(bad))); err == nil {
			t.Fatal("truncated ZIP fixture remained readable")
		}
		return bad
	case "wrong episode member":
		return zipSubSourceFiles(t, map[string][]byte{"Show.S01E03.srt": original})
	case "ambiguous exact members":
		return zipSubSourceFiles(t, map[string][]byte{"Show.S01E02.srt": original, "Alternate.Show.S01E02.srt": original})
	case "body above four MiB":
		return bytes.Repeat([]byte("x"), (4<<20)+1)
	default:
		return good
	}
}

func corruptSubSourceFallbackArchive(t *testing.T, good []byte) []byte {
	t.Helper()
	bad := bytes.Clone(good)
	header := bytes.Index(bad, []byte{'P', 'K', 1, 2})
	if header < 0 {
		t.Fatal("central ZIP header missing")
	}
	checksum := binary.LittleEndian.Uint32(bad[header+16 : header+20])
	binary.LittleEndian.PutUint32(bad[header+16:header+20], checksum^1)
	archive, err := zip.NewReader(bytes.NewReader(bad), int64(len(bad)))
	if err != nil {
		t.Fatal(err)
	}
	member, err := archive.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(member)
	_ = member.Close()
	if !errors.Is(err, zip.ErrChecksum) {
		t.Fatalf("CRC fixture must fail checksum validation: %v", err)
	}
	return bad
}

type subSourceAcquisitionEffects struct {
	sidecarPresent   bool
	sidecar          []byte
	originals        [][]byte
	records, history int
	extraMediaFiles  int
}

func (effects subSourceAcquisitionEffects) empty() bool {
	return !effects.sidecarPresent && effects.extraMediaFiles == 0 && len(effects.originals) == 0 && effects.records == 0 && effects.history == 0
}

func readSubSourceAcquisitionEffects(media, data string) (subSourceAcquisitionEffects, error) {
	var effects subSourceAcquisitionEffects
	var err error
	effects.sidecar, err = os.ReadFile(filepath.Join(filepath.Dir(media), "Show.S01E02.en.srt"))
	if err != nil && !os.IsNotExist(err) {
		return effects, err
	}
	effects.sidecarPresent = err == nil
	mediaFiles, err := os.ReadDir(filepath.Dir(media))
	if err != nil {
		return effects, err
	}
	for _, entry := range mediaFiles {
		if entry.Name() != filepath.Base(media) {
			effects.extraMediaFiles++
		}
	}
	effects.records, effects.history, err = readSubSourceAcquisitionLedger(data)
	if err != nil {
		return effects, err
	}

	effects.originals, err = readSubSourceAcquisitionOriginals(data)
	return effects, err
}

func readSubSourceAcquisitionLedger(data string) (int, int, error) {
	encoded, err := os.ReadFile(filepath.Join(data, "subtitle_acquisitions.json"))
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	var ledger struct {
		Records map[string]json.RawMessage
		History []json.RawMessage
	}
	if err := json.Unmarshal(encoded, &ledger); err != nil {
		return 0, 0, err
	}
	return len(ledger.Records), len(ledger.History), nil
}

func readSubSourceAcquisitionOriginals(data string) ([][]byte, error) {
	var result [][]byte
	directory := filepath.Join(data, "subtitle-originals")
	originals, err := os.ReadDir(directory)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range originals {
		original, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		result = append(result, original)
	}
	return result, nil
}
