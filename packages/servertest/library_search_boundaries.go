package servertest

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func (fixture LibraryAPIFixture) ShortSearchMatchesTitlesAndMetadataWordStarts(t *testing.T) {
	mediaDir := t.TempDir()
	for name, content := range map[string]string{
		"Example Movie.mp4": "video",
		"Index.mp4":         "video",
		"Quiet Harbor.mp4":  "video",
		"Quiet Harbor.nfo":  `<movie><title>Quiet Harbor</title><plot>The next day brings a complex text.</plot><director>Alex North</director></movie>`,
		"Ocean.mp4":         "video",
		"Ocean.nfo":         `<movie><title>Ocean</title><plot>An ex-soldier returns home.</plot></movie>`,
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	response := APICall(t, handler, "", http.MethodGet, "/api/v1/library?q=ex", nil)
	var catalog struct {
		Items []struct{ Title string } `json:"items"`
		Total int                      `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || response.Code != http.StatusOK {
		t.Fatalf("search = %d %q: %v", response.Code, response.Body.String(), err)
	}
	titles := make([]string, 0, len(catalog.Items))
	for _, item := range catalog.Items {
		titles = append(titles, item.Title)
	}
	if want := []string{"Example Movie", "Index", "Ocean"}; !slices.Equal(titles, want) || catalog.Total != len(want) {
		t.Fatalf("search titles = %q total %d, want %q", titles, catalog.Total, want)
	}
	director := APICall(t, handler, "", http.MethodGet, "/api/v1/library?q=alex", nil)
	AssertAPIBody(t, director, http.StatusOK, `"title":"Quiet Harbor"`, `"total":1`)
}
