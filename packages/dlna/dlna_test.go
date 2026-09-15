package dlna

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type dlnaFixture struct {
	token string
	name  string
	items []library.Item
	safe  map[string]bool
}

func (fixture *dlnaFixture) settings() SettingsFuncs {
	return SettingsFuncs{TokenFunc: func() string { return fixture.token }, ServerNameFunc: func() string { return fixture.name }}
}

func (fixture *dlnaFixture) library() LibraryFuncs {
	return LibraryFuncs{
		SnapshotFunc: func() ([]library.Item, error) { return fixture.items, nil },
		FindFunc: func(id string) (library.Item, bool) {
			for _, item := range fixture.items {
				if item.ID == id {
					return item, true
				}
			}
			return library.Item{}, false
		},
		SafeFunc: func(path string) bool { return fixture.safe[path] },
	}
}

func TestRegisterServesDeviceBrowseAndMedia(t *testing.T) { //nolint:cyclop // One fixture verifies the related DLNA protocol surface.
	t.Parallel()
	root := t.TempDir()
	video := filepath.Join(root, "film.mp4")
	if err := os.WriteFile(video, []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := &dlnaFixture{
		token: "secret", name: `Home & Cinema`,
		items: []library.Item{
			{ID: "film", Kind: "video", Title: `Film & Friends`, Container: "MP4", Path: video},
			{ID: "book", Kind: "book", Title: "Book", Path: filepath.Join(root, "book.epub")},
			{ID: "outside", Kind: "audio", Title: "Outside", Path: filepath.Join(root, "outside.mp3")},
		},
		safe: map[string]bool{video: true},
	}
	mux := http.NewServeMux()
	notFound := func(writer http.ResponseWriter, _ *http.Request) { http.Error(writer, "missing", http.StatusNotFound) }
	if err := Register(mux, context.Background(), Config{Base: "invalid", Settings: fixture.settings(), Library: fixture.library(), NotFound: notFound}); err != nil {
		t.Fatal(err)
	}

	assertResponse(t, mux, http.MethodGet, "/dlna/secret/device.xml", "", "", http.StatusOK, "Home &amp; Cinema", "/dlna/secret/content")
	assertResponse(t, mux, http.MethodGet, "/dlna/wrong/device.xml", "", "", http.StatusNotFound, "missing")
	assertResponse(t, mux, http.MethodGet, "/dlna/secret/content.xml", "", "", http.StatusOK, "<name>Browse</name>")
	assertResponse(t, mux, http.MethodGet, "/dlna/wrong/content.xml", "", "", http.StatusNotFound, "missing")
	assertResponse(t, mux, http.MethodPost, "/dlna/secret/content", "SOAPAction", contentDirectory+"#Browse", http.StatusOK, "Film &amp;amp; Friends", "/dlna/secret/media/film")
	assertResponse(t, mux, http.MethodPost, "/dlna/secret/content", "SOAPAction", "unknown", http.StatusNotFound, "missing")
	assertResponse(t, mux, http.MethodPost, "/dlna/secret/content", "SOAPAction", strings.Repeat("x", 257)+contentDirectory+"#Browse", http.StatusNotFound, "missing")
	assertResponse(t, mux, http.MethodGet, "/dlna/secret/media/film", "", "", http.StatusOK, "movie")
	assertResponse(t, mux, http.MethodHead, "/dlna/secret/media/film", "", "", http.StatusOK)
	assertResponse(t, mux, http.MethodGet, "/dlna/secret/media/outside", "", "", http.StatusNotFound, "missing")
	assertResponse(t, mux, http.MethodGet, "/dlna/secret/media/unknown", "", "", http.StatusNotFound, "missing")
}

func assertResponse(t *testing.T, handler http.Handler, method, path, header, value string, status int, contains ...string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	if header != "" {
		request.Header.Set(header, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("%s %s = %d %q", method, path, response.Code, response.Body.String())
	}
	for _, expected := range contains {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("%s %s missing %q: %q", method, path, expected, response.Body.String())
		}
	}
}

func TestDLNAValidationAndClassification(t *testing.T) {
	t.Parallel()
	for value, valid := range map[string]bool{
		"http://media.test": true, "https://media.test/path": true,
		"": false, "ftp://media.test": false, "http://": false, "http://user@media.test": false,
		"http://media.test?x=1": false, "http://media.test#x": false, "http://media.test\nwrong": false,
		"http://media.test/" + strings.Repeat("x", 2048): false,
	} {
		if ValidURL(value) != valid {
			t.Errorf("ValidURL(%q) = %t", value, !valid)
		}
	}
	for kind, expected := range map[string]string{
		"audio": "object.item.audioItem.musicTrack", "photo": "object.item.imageItem.photo", "video": "object.item.videoItem.movie",
	} {
		if actual := Class(library.Item{Kind: kind}); actual != expected {
			t.Errorf("Class(%q) = %q", kind, actual)
		}
	}
	response := SSDPResponse("http://media.test/", "token")
	if !strings.Contains(response, "LOCATION: http://media.test/dlna/token/device.xml") || !strings.Contains(response, "USN: uuid:") {
		t.Fatalf("SSDP response = %q", response)
	}
}

func TestRegisterRejectsMissingAdapters(t *testing.T) {
	t.Parallel()
	if err := Register(nil, nil, Config{}); err == nil {
		t.Fatal("missing adapters were accepted")
	}
}

type fakeSSDPConnection struct {
	message string
	err     error
	wrote   string
	cancel  context.CancelFunc
	closed  bool
}

func (*fakeSSDPConnection) SetReadDeadline(time.Time) error { return nil }

func (connection *fakeSSDPConnection) Close() error {
	connection.closed = true
	return nil
}

func (connection *fakeSSDPConnection) ReadFromUDP(buffer []byte) (int, *net.UDPAddr, error) {
	length := copy(buffer, connection.message)
	connection.cancel()
	return length, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1900}, connection.err
}

func (connection *fakeSSDPConnection) WriteToUDP(data []byte, _ *net.UDPAddr) (int, error) {
	connection.wrote = string(data)
	return len(data), nil
}

func TestSSDPConnectionRecognizesOnlyValidSearches(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, message string
		err           error
		writes        bool
	}{
		{"recognized", "M-SEARCH * HTTP/1.1\r\nST: ssdp:all\r\n", nil, true},
		{"unknown", "M-SEARCH * HTTP/1.1\r\nST: other\r\n", nil, false},
		{"read failure", "M-SEARCH * HTTP/1.1\r\nST: ssdp:all\r\n", errors.New("read"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			connection := &fakeSSDPConnection{message: test.message, err: test.err, cancel: cancel}
			serveSSDPConnection(ctx, "http://media.test", SettingsFuncs{TokenFunc: func() string { return "token" }, ServerNameFunc: func() string { return "Kinosail" }}, connection)
			if (connection.wrote != "") != test.writes {
				t.Fatalf("SSDP write = %q", connection.wrote)
			}
		})
	}
}

func TestRegisterStartsDiscoveryAndSSDPClosesItsConnection(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	discovered := make(chan string, 1)
	config := Config{
		Base: "http://media.test", Settings: (&dlnaFixture{token: "token"}).settings(), Library: (&dlnaFixture{}).library(),
		NotFound: func(http.ResponseWriter, *http.Request) {},
		discover: func(_ context.Context, base string, _ Settings) { discovered <- base },
	}
	if err := Register(http.NewServeMux(), ctx, config); err != nil {
		t.Fatal(err)
	}
	select {
	case base := <-discovered:
		if base != config.Base {
			t.Fatalf("discovery base = %q", base)
		}
	case <-time.After(time.Second):
		t.Fatal("discovery did not start")
	}
	cancel()
	serveSSDP(ctx, config.Base, config.Settings)
	config.discover = nil
	if err := Register(http.NewServeMux(), ctx, config); err != nil {
		t.Fatal(err)
	}

	connection := &fakeSSDPConnection{message: "M-SEARCH * HTTP/1.1\r\nST: ssdp:all\r\n", cancel: func() { cancel() }}
	serveSSDPWith(ctx, config.Base, config.Settings, func(network string, address *net.UDPAddr) (ssdpConnection, error) {
		if network != "udp4" || address.String() != "239.255.255.250:1900" {
			t.Fatalf("listen = %q, %v", network, address)
		}
		return connection, nil
	})
	if !connection.closed {
		t.Fatal("SSDP connection was not closed")
	}
	serveSSDPWith(t.Context(), config.Base, config.Settings, func(string, *net.UDPAddr) (ssdpConnection, error) {
		return nil, errors.New("listen failed")
	})
}
