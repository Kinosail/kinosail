package server

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
)

func TestSubtitleProviderDownloadAndURLSafetyBranches(t *testing.T) { //nolint:cyclop // One fixture proves every download boundary and URL form.
	t.Parallel()
	valid := []byte("1\n00:00:01,000 --> 00:00:02,000\nDialogue\n")
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/ok":
			_, _ = writer.Write(valid)
		case "/invalid":
			_, _ = writer.Write([]byte("not a subtitle"))
		case "/redirect":
			http.Redirect(writer, request, "https://untrusted.example/subtitle.srt", http.StatusFound)
		default:
			http.Error(writer, "unavailable", http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(remote.Close)
	newProvider := func() *subtitleProvider {
		return newSubtitleProvider(SubtitleConfig{URL: remote.URL + "/api/v1", APIKey: "key"}, t.TempDir(), t.TempDir(), nil, nil, "")
	}
	provider := newProvider()
	if data, err := provider.download(t.Context(), "/ok"); err != nil || !bytes.Equal(data, valid) {
		t.Fatalf("plain download = %q, %v", data, err)
	}
	if _, err := provider.download(t.Context(), "https://untrusted.example/subtitle.srt"); err == nil {
		t.Fatal("untrusted download was accepted")
	}
	if _, err := newProvider().download(t.Context(), "/invalid"); err == nil {
		t.Fatal("invalid subtitle payload was accepted")
	}
	if _, err := newProvider().download(t.Context(), "/unavailable"); err == nil {
		t.Fatal("failed subtitle response was accepted")
	}
	var response map[string]any
	if err := newProvider().json(t.Context(), remote.URL+"/redirect", &response); err == nil {
		t.Fatal("untrusted provider redirect was accepted")
	}

	if provider.resolve("://bad") != "://bad" || (&subtitleProvider{config: SubtitleConfig{URL: "://bad"}}).resolve("file.srt") != "file.srt" {
		t.Fatal("invalid URL resolution changed the input")
	}
	public := &subtitleProvider{config: SubtitleConfig{URL: "https://api.subdl.com/api/v1"}}
	if resolved := public.resolve("downloads/file.srt"); resolved != "https://dl.subdl.com/downloads/file.srt" {
		t.Fatalf("SubDL relative URL = %q", resolved)
	}
	if public.allowed("http://api.subdl.com/file") || !public.allowed("https://dl.subdl.com/file") || public.allowed("://bad") {
		t.Fatal("provider host policy accepted an unsafe URL")
	}
}

func TestSubtitleProviderArchiveAndResponseFailureBranches(t *testing.T) {
	t.Parallel()
	response := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(iotest.ErrReader(errors.New("read failed")))}
	if _, err := readSubtitle(response); err == nil {
		t.Fatal("response read failure was accepted")
	}
	response = &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}
	if _, err := readSubtitle(response); err == nil {
		t.Fatal("non-OK response was accepted")
	}

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, name := range []string{"one.srt", "two.vtt"} {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = file.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nDialogue\n"))
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = readArchivedSubtitle(t.Context(), reader); err == nil {
		t.Fatal("ambiguous subtitle archive was accepted")
	}
}
