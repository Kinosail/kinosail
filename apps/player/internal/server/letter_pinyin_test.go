package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

type letterPage struct {
	Items   []struct{ Title string } `json:"items"`
	Letter  string                   `json:"letter"`
	Letters []struct {
		Label string `json:"label"`
		Count int    `json:"count"`
	} `json:"letters"`
}

func chineseLibrary(t *testing.T) http.Handler {
	t.Helper()
	media := t.TempDir()
	for _, name := range []string{"阿凡达 (2009)", "爱情公寓 (2018)", "八佰 (2020)", "超人 (2025)", "大话西游 (1995)", "疯狂动物城2 (2025)", "葫芦兄弟 (1986)", "僵尸先生 (1985)", "Avatar (2009)", "Batman (2022)", "Parasite (2019)", "Titanic (1997)", "1917 (2019)"} {
		if err := os.WriteFile(filepath.Join(media, name+".mp4"), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return server.New(server.Config{MediaDir: media, DataDir: t.TempDir()})
}

func libraryLetters(t *testing.T, handler http.Handler, query string) letterPage {
	t.Helper()
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&"+query, nil)
	var page letterPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK {
		t.Fatalf("%s = %d %v %q", query, response.Code, err, response.Body.String())
	}
	return page
}

func letterSummary(page letterPage) string {
	var labels []string
	for _, letter := range page.Letters {
		labels = append(labels, letter.Label+":"+strings.Repeat("|", letter.Count))
	}
	return strings.Join(labels, " ")
}

func TestChineseLetterJumpUsesPinyinInitials(t *testing.T) {
	t.Parallel()
	handler := chineseLibrary(t)
	const want = "#:| A:||| B:|| C:| D:| F:| H:| J:| P:| T:|"
	for _, language := range []string{"zh-Hans", "zh-Hant", "zh-CN", "zh-TW"} {
		if got := letterSummary(libraryLetters(t, handler, "lang="+language)); got != want {
			t.Fatalf("%s letter buckets = %q, want %q", language, got, want)
		}
	}
	page := libraryLetters(t, handler, "lang=zh-Hans&letter=a")
	var titles []string
	for _, item := range page.Items {
		titles = append(titles, item.Title)
	}
	if page.Letter != "A" || strings.Join(titles, ",") != "Avatar,阿凡达,爱情公寓" {
		t.Fatalf("pinyin A bucket = %q %q", page.Letter, titles)
	}
	if page := libraryLetters(t, handler, "lang=zh-Hans&letter=J"); len(page.Items) != 1 || page.Items[0].Title != "僵尸先生" {
		t.Fatalf("pinyin J bucket = %+v", page.Items)
	}
	if page := libraryLetters(t, handler, "lang=zh-Hans&letter=阿"); page.Letter != "A" || len(page.Items) != 3 {
		t.Fatalf("a Han character letter must select its pinyin bucket: %+v", page)
	}

	web := httptest.NewRecorder()
	handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&lang=zh-Hans", nil))
	body := web.Body.String()
	start := strings.Index(body, `<nav class="letter-jump"`)
	if web.Code != http.StatusOK || start < 0 {
		t.Fatalf("web letter jump missing: %d", web.Code)
	}
	end := strings.Index(body[start:], `</nav>`)
	var rendered []string
	for _, match := range regexp.MustCompile(`data-title-letter="([^"]+)"`).FindAllStringSubmatch(body[start:start+end], -1) {
		rendered = append(rendered, match[1])
	}
	if strings.Join(rendered, "") != "#ABCDFHJPT" {
		t.Fatalf("web letter jump = %q", rendered)
	}
}

func TestEnglishLetterJumpKeepsChineseCharactersSeparate(t *testing.T) {
	t.Parallel()
	summary := letterSummary(libraryLetters(t, chineseLibrary(t), "lang=en"))
	if !strings.HasPrefix(summary, "#:| A:| B:| P:| T:| ") || !strings.Contains(summary, "阿:|") || !strings.Contains(summary, "僵:|") {
		t.Fatalf("English letter buckets changed: %q", summary)
	}
}
