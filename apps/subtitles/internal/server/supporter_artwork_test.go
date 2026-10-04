package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
