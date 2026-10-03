package federation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRemoteSAMLMetadataRejectsOversizedResponse(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxSAMLMetadata+1)))
	}))
	defer remote.Close()
	if metadata, err := fetchSAMLMetadata(t.Context(), remote.URL); err == nil || metadata != nil {
		t.Fatal("oversized metadata accepted")
	}
}
