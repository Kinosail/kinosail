package dlna

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func browseFixtureMux(t *testing.T) (*http.ServeMux, *int) {
	t.Helper()
	safeCalls := 0
	fixture := &dlnaFixture{token: "secret", name: "Kinosail", items: []library.Item{
		{ID: "one", Kind: "video", Title: `One & "Only"`, Container: "MKV", Path: "one"},
		{ID: "outside", Kind: "video", Title: "Outside", Container: "MP4", Path: "outside"},
		{ID: "book", Kind: "book", Title: "Book", Path: "book"},
		{ID: "two", Kind: "audio", Title: "Two <Live>", Container: "MP3", Path: "two"},
		{ID: "three", Kind: "photo", Title: "Three's", Container: "JPG", Path: "three"},
		{ID: "four", Kind: "video", Title: "Four", Container: "MP4", Path: "four"},
		{ID: "five", Kind: "video", Title: "Five", Container: "WEBM", Path: "five"},
	}, safe: map[string]bool{"one": true, "book": true, "two": true, "three": true, "four": true, "five": true}}
	adapter := fixture.library()
	snapshot, _ := adapter.Library()
	adapter.LibraryFunc = func() (*library.Snapshot, error) { return snapshot, nil }
	safe := adapter.SafeFunc
	adapter.SafeFunc = func(path string) bool { safeCalls++; return safe(path) }
	mux := http.NewServeMux()
	if err := Register(mux, context.Background(), Config{Base: "http://media.test/", Settings: fixture.settings(), Library: adapter, NotFound: http.NotFound}); err != nil {
		t.Fatal(err)
	}
	return mux, &safeCalls
}

func browseCall(t *testing.T, mux http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/dlna/secret/content", strings.NewReader(body))
	request.Header.Set("SOAPAction", `"`+contentDirectory+`#Browse"`)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func TestBrowseReturnsTheRequestedPage(t *testing.T) {
	t.Parallel()
	mux, safeCalls := browseFixtureMux(t)
	page := browseCall(t, mux, browseEnvelope("0", "BrowseDirectChildren", 1, 2))
	want := `<Result>&lt;DIDL-Lite xmlns=&#34;urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/&#34; xmlns:dc=&#34;http://purl.org/dc/elements/1.1/&#34; xmlns:upnp=&#34;urn:schemas-upnp-org:metadata-1-0/upnp/&#34;&gt;` +
		`&lt;item id=&#34;two&#34; parentID=&#34;0&#34; restricted=&#34;1&#34;&gt;&lt;dc:title&gt;Two &amp;lt;Live&amp;gt;&lt;/dc:title&gt;&lt;upnp:class&gt;object.item.audioItem.musicTrack&lt;/upnp:class&gt;&lt;res protocolInfo=&#34;http-get:*:audio/mpeg:*&#34;&gt;http://media.test/dlna/secret/media/two&lt;/res&gt;&lt;/item&gt;` +
		`&lt;item id=&#34;three&#34; parentID=&#34;0&#34; restricted=&#34;1&#34;&gt;&lt;dc:title&gt;Three&amp;#39;s&lt;/dc:title&gt;&lt;upnp:class&gt;object.item.imageItem.photo&lt;/upnp:class&gt;&lt;res protocolInfo=&#34;http-get:*:image/jpeg:*&#34;&gt;http://media.test/dlna/secret/media/three&lt;/res&gt;&lt;/item&gt;` +
		`&lt;/DIDL-Lite&gt;</Result><NumberReturned>2</NumberReturned><TotalMatches>5</TotalMatches>`
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), want) {
		t.Fatalf("page = %d %q, safe checks = %d", page.Code, page.Body.String(), *safeCalls)
	}
	for _, test := range []struct {
		body, contains string
	}{
		{"", `<NumberReturned>5</NumberReturned><TotalMatches>5</TotalMatches>`},
		{browseEnvelope("0", "BrowseDirectChildren", 0, 0), `One &amp;amp; &amp;#34;Only&amp;#34;`},
		{browseEnvelope("0", "BrowseDirectChildren", 4, 10), `media/five&lt;/res&gt;&lt;/item&gt;&lt;/DIDL-Lite&gt;</Result><NumberReturned>1</NumberReturned><TotalMatches>5</TotalMatches>`},
		{browseEnvelope("0", "BrowseDirectChildren", 9, 10), `<NumberReturned>0</NumberReturned><TotalMatches>5</TotalMatches>`},
		{browseEnvelope("0", "BrowseMetadata", 0, 0), `&lt;container id=&#34;0&#34; parentID=&#34;-1&#34; restricted=&#34;1&#34; childCount=&#34;5&#34;&gt;&lt;dc:title&gt;Kinosail&lt;/dc:title&gt;&lt;upnp:class&gt;object.container.storageFolder&lt;/upnp:class&gt;&lt;/container&gt;&lt;/DIDL-Lite&gt;</Result><NumberReturned>1</NumberReturned><TotalMatches>1</TotalMatches>`},
		{browseEnvelope("four", "BrowseMetadata", 0, 0), `media/four&lt;/res&gt;&lt;/item&gt;&lt;/DIDL-Lite&gt;</Result><NumberReturned>1</NumberReturned><TotalMatches>1</TotalMatches>`},
	} {
		response := browseCall(t, mux, test.body)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.contains) || strings.Contains(response.Body.String(), "Book") || strings.Contains(response.Body.String(), "Outside") {
			t.Fatalf("browse %q = %d %q", test.body, response.Code, response.Body.String())
		}
	}
	if *safeCalls > 7 {
		t.Fatalf("seven Browse requests ran %d path checks; want one per playable item per snapshot", *safeCalls)
	}
}

func TestBrowseRejectsInvalidArguments(t *testing.T) {
	t.Parallel()
	mux, _ := browseFixtureMux(t)
	for body, code := range map[string]string{
		browseEnvelope("0", "BrowseDirectChildren", -1, 2):                                    "402",
		strings.Replace(browseEnvelope("0", "BrowseDirectChildren", 0, 2), ">2<", ">two<", 1): "402",
		browseEnvelope("0", "BrowseSideways", 0, 0):                                           "402",
		`<s:Envelope><s:Body><u:Browse><ObjectID>0</ObjectID>`:                                "402",
		browseEnvelope("0", "BrowseDirectChildren", 0, 0) + strings.Repeat(" ", 70_000):       "402",
		browseEnvelope("missing", "BrowseMetadata", 0, 0):                                     "701",
		browseEnvelope("book", "BrowseMetadata", 0, 0):                                        "701",
		browseEnvelope("outside", "BrowseMetadata", 0, 0):                                     "701",
		browseEnvelope("four", "BrowseDirectChildren", 0, 0):                                  "710",
		browseEnvelope(strings.Repeat("x", 300), "BrowseMetadata", 0, 0):                      "402",
	} {
		response := browseCall(t, mux, body)
		if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "<errorCode>"+code+"</errorCode>") || strings.Contains(response.Body.String(), "DIDL") {
			t.Fatalf("browse %.80q = %d %q, want UPnP error %s", body, response.Code, response.Body.String(), code)
		}
	}
}
