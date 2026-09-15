package server

import "testing"

func TestSubtitleProviderResolvesRelativeDownloadLinks(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		base string
		link string
		want string
	}{
		"absolute":        {"https://api.subdl.com/api/v1", "https://dl.subdl.com/file.srt", "https://dl.subdl.com/file.srt"},
		"invalid link":    {"https://api.subdl.com/api/v1", "%", "%"},
		"invalid base":    {"://invalid", "file.srt", "file.srt"},
		"SubDL download":  {"https://api.subdl.com/api/v1", "/file.srt", "https://dl.subdl.com/file.srt"},
		"custom endpoint": {"http://127.0.0.1:8080/api/v1", "file.srt", "http://127.0.0.1:8080/file.srt"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			provider := subtitleProvider{config: SubtitleConfig{URL: test.base}}
			if got := provider.resolve(test.link); got != test.want {
				t.Fatalf("resolved link = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSubtitleProviderAllowsOnlyConfiguredOrSubDLOrigins(t *testing.T) {
	t.Parallel()
	provider := subtitleProvider{config: SubtitleConfig{URL: "https://api.subdl.com/api/v1"}}
	for link, allowed := range map[string]bool{
		"%":                               false,
		"http://api.subdl.com/file.srt":   false,
		"https://api.subdl.com/file.srt":  true,
		"https://dl.subdl.com/file.srt":   true,
		"https://subdl.com.evil/file.srt": false,
	} {
		if got := provider.allowed(link); got != allowed {
			t.Fatalf("link %q allowed = %v", link, got)
		}
	}
	provider.config.URL = "://invalid"
	if provider.allowed("https://dl.subdl.com/file.srt") {
		t.Fatal("invalid configured origin allowed a subtitle link")
	}
}
