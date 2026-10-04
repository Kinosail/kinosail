package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// The populated browser fixture has readable media. This HTTP regression creates
// a failed track check and follows its rendered recovery link to real Settings.
func TestSubtitleUnavailableRecoveryReachesMediaLibraries(t *testing.T) {
	t.Parallel()
	manager, _ := subtitleFactsFixture(t, "#!/bin/sh\nexit 1\n")
	manager.refreshFacts(t.Context())
	page := httptest.NewRecorder()
	manager.dashboard(page, ownerRequest("/?view=library&status=unavailable"))
	if page.Code != http.StatusOK {
		t.Fatalf("unavailable library = %d", page.Code)
	}
	root := subtitleRenderedHTML(t, page.Body.String())
	notice := subtitleRenderedElement(t, root, "p", "class", "subtitle-background-note")
	link := subtitleRenderedElement(t, notice, "a", "", "")
	destination, err := url.Parse(subtitleHTMLAttribute(link, "href"))
	if err != nil {
		t.Fatal(err)
	}
	if destination.Path != "/settings" || destination.Fragment == "" {
		t.Fatalf("file-access recovery lacks a Settings section: %v", destination)
	}
	settings := httptest.NewRecorder()
	showSubtitleSettings(manager.settings, manager.provider, nil)(settings, ownerRequest(destination.Path))
	if settings.Code != http.StatusOK {
		t.Fatalf("recovery destination = %d", settings.Code)
	}
	root = subtitleRenderedHTML(t, settings.Body.String())
	section := subtitleRenderedElement(t, root, "section", "id", destination.Fragment)
	heading := subtitleRenderedElement(t, section, "h2", "", "")
	if heading.FirstChild == nil || heading.FirstChild.Data != "Media Libraries" {
		t.Fatalf("rendered recovery link %q does not resolve to Media Libraries", destination.String())
	}
}

func subtitleRenderedHTML(t *testing.T, body string) *html.Node {
	t.Helper()
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func subtitleRenderedElement(t *testing.T, root *html.Node, tag, attribute, value string) *html.Node {
	t.Helper()
	for node := range root.Descendants() {
		if node.Type == html.ElementNode && node.Data == tag && subtitleHTMLAttribute(node, attribute) == value {
			return node
		}
	}
	t.Fatalf("rendered %s with %s=%q is missing", tag, attribute, value)
	return nil
}
