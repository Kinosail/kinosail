package server_test

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestSupporterBadgeArtworkIsAvailableForEveryLevel(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	page := apiCall(t, handler, token, http.MethodGet, "/supporter", nil)
	if page.Code != http.StatusOK {
		t.Fatalf("supporter page = %d", page.Code)
	}
	for _, family := range []string{"living-standard", "patron-order"} {
		for rank := 1; rank <= 10; rank++ {
			asset := "/static/supporter/badges/" + family + "-" + strconv.Itoa(rank) + ".svg"
			if !strings.Contains(page.Body.String(), `src="`+asset+`?v=`) {
				t.Errorf("supporter page does not display %s", asset)
			}
			checkSupporterBadgeAsset(t, handler, token, asset)
		}
	}
}

func checkSupporterBadgeAsset(t *testing.T, handler http.Handler, token, asset string) {
	t.Helper()
	response := apiCall(t, handler, token, http.MethodGet, asset, nil)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("badge %s = %d, %q", asset, response.Code, response.Header().Get("Content-Type"))
	}
	var document struct {
		Title string `xml:"title"`
	}
	if err := xml.Unmarshal(response.Body.Bytes(), &document); err != nil || !strings.Contains(document.Title, "Subtitles") {
		t.Errorf("badge %s is not titled Subtitles: %q, %v", asset, document.Title, err)
	}
}

func TestSupporterBadgeArtworkRejectsInvalidNames(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	for _, name := range []string{"living-standard-0.svg", "patron-order-11.svg", "other-1.svg", strings.Repeat("x", 256) + ".svg"} {
		response := apiCall(t, handler, token, http.MethodGet, "/static/supporter/badges/"+name, nil)
		if response.Code != http.StatusNotFound {
			t.Errorf("invalid badge %q = %d", name, response.Code)
		}
	}
}
