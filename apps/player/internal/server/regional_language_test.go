package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func regionalLanguageHandler(t *testing.T) http.Handler {
	t.Helper()
	media := t.TempDir()
	for _, name := range []string{"Arrival (2016).mp4", "Heat (1995).mp4", filepath.Join("Test Show", "Season 1", "Test Show S01E01.mp4")} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(media, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(media, name), []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler, _ := firstWebItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir()})
	return handler
}

func regionalPage(t *testing.T, handler http.Handler, path, acceptLanguage string) (string, http.Header) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := serveRequest(handler, request)
	if response.Code != http.StatusOK {
		t.Fatalf("%s (%s) = %d", path, acceptLanguage, response.Code)
	}
	return response.Body.String(), response.Header()
}

func TestRegionalChineseUsesTheMatchingScriptTranslation(t *testing.T) {
	t.Parallel()
	handler := regionalLanguageHandler(t)
	for _, tc := range []struct {
		path, accept, tag string
		want              []string
	}{
		{"/?lang=zh-CN", "", "zh-CN", []string{"电影", "设置", "媒体库"}},
		{"/?lang=zh-TW", "", "zh-TW", []string{"電影", "設定", "媒體庫"}},
		{"/?lang=es-MX", "", "es-MX", []string{"Películas", "Configuración"}},
		{"/", "zh-CN,zh;q=0.9", "zh-CN", []string{"电影", "设置"}},
		{"/", "zh-TW", "zh-Hant", []string{"電影", "設定", "媒體庫"}},
		{"/", "zh-HK", "zh-Hant", []string{"電影", "設定"}},
		{"/", "es-MX", "es-MX", []string{"Películas", "Configuración"}},
		{"/", "pt", "pt", []string{"Filmes", "Configurações"}},
	} {
		body, header := regionalPage(t, handler, tc.path, tc.accept)
		if header.Get("Content-Language") != tc.tag || !strings.Contains(body, `lang="`+tc.tag+`"`) {
			t.Errorf("%s %s: Content-Language %q", tc.path, tc.accept, header.Get("Content-Language"))
		}
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Errorf("%s %s lacks %q", tc.path, tc.accept, want)
			}
		}
		if strings.Contains(body, ">Settings<") || strings.Contains(body, ">Movies<") {
			t.Errorf("%s %s still shows English navigation", tc.path, tc.accept)
		}
	}
	english, _ := regionalPage(t, handler, "/", "en-GB")
	if !strings.Contains(english, ">Settings<") {
		t.Error("English regional page lost English navigation")
	}
}

func TestChinesePagesJoinWordsWithoutSpacesOrEnglishPlurals(t *testing.T) {
	t.Parallel()
	handler := regionalLanguageHandler(t)
	spaced := regexp.MustCompile(`>[^<]*\p{Han} +\p{Han}[^<]*<`)
	for _, tag := range []string{"zh-Hans", "zh-Hant", "zh-CN"} {
		for _, path := range []string{"/", "/?view=movies", "/?view=shows", "/?view=playlists", "/settings", "/language"} {
			body, _ := regionalPage(t, handler, path+map[bool]string{true: "&", false: "?"}[strings.Contains(path, "?")]+"lang="+tag, "")
			if match := spaced.FindString(body); match != "" {
				t.Errorf("%s %s separates Chinese words with a space: %q", tag, path, match)
			}
		}
		movies, _ := regionalPage(t, handler, "/?view=movies&lang="+tag, "")
		if !strings.Contains(movies, "<strong>2</strong> 项") && !strings.Contains(movies, "<strong>2</strong> 項") || strings.Contains(movies, "项s") || strings.Contains(movies, "項s") || strings.Contains(movies, "</strong> items") {
			t.Errorf("%s movie count is not a Chinese count", tag)
		}
	}
	english, _ := regionalPage(t, handler, "/?view=movies", "en")
	if !strings.Contains(english, "<strong>2</strong> items") {
		t.Error("English movie count lost its plural")
	}
}

func TestChineseSettingsPagesTranslateServerBuiltLabels(t *testing.T) {
	t.Parallel()
	handler := regionalLanguageHandler(t)
	for _, tc := range []struct {
		path          string
		want, english []string
	}{
		{"/settings/agent-connections", []string{"连接 Codex 和其他 AI 代理"}, []string{"Connect Codex and other AI agents"}},
		{"/settings/media-shares", []string{"创建媒体分享"}, []string{">Create a Media Share<"}},
		{"/settings/configuration", []string{"备份文件夹"}, []string{">Backup folder<"}},
		{"/settings/remote-readiness", []string{"在路由器中找到您的服务器"}, []string{">Find your Server in the router<"}},
		{"/settings", []string{"关于与致谢"}, []string{"About &amp; thanks"}},
		{"/", []string{"最近添加的电影", "<small>2 项</small>"}, []string{">Recently added movies<", " items</small>"}},
	} {
		body, _ := regionalPage(t, handler, tc.path+"?lang=zh-Hans", "")
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", tc.path, want)
			}
		}
		for _, english := range tc.english {
			if strings.Contains(body, english) {
				t.Errorf("%s still shows %q", tc.path, english)
			}
		}
	}
}

func TestChineseShowAndPlayerTextUsesCatalogFormats(t *testing.T) {
	t.Parallel()
	handler := regionalLanguageHandler(t)
	home, _ := regionalPage(t, handler, "/?view=shows&lang=zh-Hans", "")
	show := regexp.MustCompile(`/show/[a-f0-9]+`).FindString(home)
	if show == "" {
		t.Fatal("shows view lacks the show fixture")
	}
	page, _ := regionalPage(t, handler, show+"?lang=zh-Hans", "")
	for _, want := range []string{"第 1 季", "播放"} {
		if !strings.Contains(page, want) {
			t.Errorf("show page lacks %q", want)
		}
	}
	for _, english := range []string{">Season 1<", "Play again", "Play episode"} {
		if strings.Contains(page, english) {
			t.Errorf("show page still shows %q", english)
		}
	}
	watch := regexp.MustCompile(`/watch/[a-f0-9]+`).FindString(page)
	player, _ := regionalPage(t, handler, watch+"?lang=zh-Hans", "")
	for _, english := range []string{"Saving progress…", "Your position is not saved", "Watched status is not saved"} {
		if strings.Contains(player, english) {
			t.Errorf("player still shows %q", english)
		}
	}
}

func TestChinesePagesTranslateGoBuiltStatusAndKeepCode(t *testing.T) {
	t.Parallel()
	handler := regionalLanguageHandler(t)
	for _, tc := range []struct {
		path          string
		want, english []string
	}{
		{"/settings", []string{"myhome-subtitles.duckdns.org", "阿尔巴尼亚语 · shqip"}, []string{"myhome-字幕", "Albanian · shqip", "Household playback defaults.", "did not inspect FFmpeg", "Install an FFmpeg build with a supported", "No installed update adapter", "Use a supported DNS"}},
		{"/supporter", nil, []string{", level ", " Friend", ">Friend<", ">Collected<"}},
		{"/?view=shows", nil, []string{"episodes stacked", ">Play next<"}},
		{"/settings/remote-readiness", nil, []string{"Public certificate is valid", "Dedicated public HTTPS is configured"}},
	} {
		body, _ := regionalPage(t, handler, tc.path+map[bool]string{true: "&", false: "?"}[strings.Contains(tc.path, "?")]+"lang=zh-Hans", "")
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", tc.path, want)
			}
		}
		for _, english := range tc.english {
			if strings.Contains(body, english) {
				t.Errorf("%s still shows %q", tc.path, english)
			}
		}
	}
	english, _ := regionalPage(t, handler, "/settings", "en")
	for _, want := range []string{"Albanian · shqip", "Household playback defaults.", "myhome-subtitles.duckdns.org"} {
		if !strings.Contains(english, want) {
			t.Errorf("English settings lost %q", want)
		}
	}
}
