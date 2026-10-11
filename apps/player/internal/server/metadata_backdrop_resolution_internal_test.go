package server

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/metadata"
)

// This isolated check covers canonical-provider variant selection. The public
// refresh journey uses a custom loopback provider and cannot reach this branch
// without a real provider account or changing the production network policy.
func TestMetadataBackdropsUseOriginalVariantWithoutChangingPostersOrCustomBases(t *testing.T) {
	t.Parallel()
	poster := backdropResolutionJPEG(t, 780, 1170)
	backdrop := backdropResolutionJPEG(t, 3840, 2160)
	for _, test := range []struct {
		name, base, posterPath, backdropPath string
	}{
		{"canonical default", "https://image.tmdb.org/t/p/w780", "/t/p/w780/poster.jpg", "/t/p/original/backdrop.jpg"},
		{"custom HTTPS", "https://images.example/custom", "/custom/poster.jpg", "/custom/backdrop.jpg"},
		{"custom loopback", "http://127.0.0.1:12345/images", "/images/poster.jpg", "/images/backdrop.jpg"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newMetadataStore(MetadataConfig{ImageURL: test.base}, "", t.TempDir())
			base, err := url.Parse(test.base)
			if err != nil {
				t.Fatal(err)
			}
			requests := 0
			store.client = &http.Client{Transport: backdropResolutionTransport(func(request *http.Request) (*http.Response, error) {
				requests++
				want, data := test.posterPath, poster
				if requests == 2 {
					want, data = test.backdropPath, backdrop
				}
				wantURL := *base
				wantURL.Path = want
				if request.Method != http.MethodGet || request.URL.String() != wantURL.String() || request.Header.Get("Authorization") != "" {
					t.Errorf("artwork request method/path/credential presence = %s %q %t", request.Method, request.URL.Path, request.Header.Get("Authorization") != "")
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/jpeg"}}, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})}
			record := metadataRecord{Title: "Movie", Artwork: store.artworkPath("movie"), Backdrop: store.artworkPath("movie-backdrop"), BackdropChecked: true}
			result := metadata.Result{Record: record, Images: []metadata.Image{
				{Source: "/poster.jpg", Target: record.Artwork},
				{Source: "/backdrop.jpg", Target: record.Backdrop, Backdrop: true},
			}}
			updated, err := store.downloadMetadataImages(t.Context(), result)
			if err != nil || requests != 2 || updated.Artwork != record.Artwork || updated.Backdrop != record.Backdrop {
				t.Fatalf("artwork result = %#v, requests=%d, err=%v", updated, requests, err)
			}
			for path, want := range map[string][]byte{record.Artwork: poster, record.Backdrop: backdrop} {
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("cached artwork changed downloaded bytes: %v", err)
				}
			}
		})
	}
}

// A successful image MIME response must not replace valid derived artwork with
// bytes that a native client cannot decode. This protects the new canonical
// high-resolution download branch without changing poster/custom-base policy.
func TestCanonicalBackdropInvalidImagePreservesExistingDerivedBytes(t *testing.T) {
	t.Parallel()
	prior := backdropResolutionJPEG(t, 780, 439)
	truncated := prior[:len(prior)/2]
	if _, err := jpeg.DecodeConfig(bytes.NewReader(truncated)); err != nil {
		t.Fatalf("truncated fixture must retain a valid image header: %v", err)
	}
	var unsupportedPNG bytes.Buffer
	if err := png.Encode(&unsupportedPNG, image.NewGray(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	oversizedDimensions := bytes.Clone(prior)
	frame := bytes.Index(oversizedDimensions, []byte{0xff, 0xc0})
	if frame < 0 || frame+9 > len(oversizedDimensions) {
		t.Fatal("JPEG fixture has no baseline frame header")
	}
	// A tiny body with a large advertised image must be bounded before decode.
	oversizedDimensions[frame+5], oversizedDimensions[frame+6] = 0x0f, 0xa0 // 4000
	oversizedDimensions[frame+7], oversizedDimensions[frame+8] = 0x17, 0x70 // 6000
	for _, test := range []struct {
		name string
		data []byte
	}{{"malformed", []byte("not an image")}, {"truncated after header", truncated},
		{"unsupported format with image MIME", unsupportedPNG.Bytes()},
		{"oversized pixel dimensions", oversizedDimensions}} {
		t.Run(test.name, func(t *testing.T) {
			store := newMetadataStore(MetadataConfig{}, "", t.TempDir())
			target := store.artworkPath("movie-backdrop")
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, prior, 0o600); err != nil {
				t.Fatal(err)
			}
			store.client = &http.Client{Transport: backdropResolutionTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/jpeg"}}, Body: io.NopCloser(bytes.NewReader(test.data))}, nil
			})}
			result := metadata.Result{Record: metadataRecord{Title: "Movie", Backdrop: target, BackdropChecked: true},
				Images: []metadata.Image{{Source: "/backdrop.jpg", Target: target, Backdrop: true}}}
			if _, err := store.downloadMetadataImages(t.Context(), result); err == nil {
				t.Error("invalid canonical backdrop was accepted")
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, prior) {
				t.Fatalf("invalid replacement changed prior derived artwork: %v", err)
			}
		})
	}
}

func TestCanonicalBackdropPixelBudgetBoundariesPreserveExistingBytes(t *testing.T) {
	prior := backdropResolutionJPEG(t, 780, 439)
	for _, test := range []struct {
		name   string
		height int
		accept bool
	}{{"ten million pixels", 2500, true}, {"above ten million pixels", 2501, false}} {
		t.Run(test.name, func(t *testing.T) {
			data := backdropResolutionJPEG(t, 4000, test.height)
			store := newMetadataStore(MetadataConfig{}, "", t.TempDir())
			target := store.artworkPath("movie-backdrop")
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, prior, 0o600); err != nil {
				t.Fatal(err)
			}
			store.client = &http.Client{Transport: backdropResolutionTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/jpeg"}}, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})}
			result := metadata.Result{Record: metadataRecord{Backdrop: target, BackdropChecked: true},
				Images: []metadata.Image{{Source: "/backdrop.jpg", Target: target, Backdrop: true}}}
			_, err := store.downloadMetadataImages(t.Context(), result)
			if (err == nil) != test.accept {
				t.Fatalf("pixel-boundary acceptance = %t, want %t", err == nil, test.accept)
			}
			want := prior
			if test.accept {
				want = data
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("pixel-boundary replacement changed expected bytes: %v", err)
			}
		})
	}
}

type backdropResolutionTransport func(*http.Request) (*http.Response, error)

func (transport backdropResolutionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func backdropResolutionJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewGray(image.Rect(0, 0, width, height)), &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
