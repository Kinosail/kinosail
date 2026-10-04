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
func TestSubtitleUnavailableRecoveryReachesMediaLibraries(t *testing.T) { //nolint:cyclop // One rendered recovery journey follows and resolves its Settings link.
	t.Parallel()
	manager, _ := subtitleFactsFixture(t, "#!/bin/sh\nexit 1\n")
	manager.refreshFacts(t.Context())
	page := httptest.NewRecorder()
	manager.dashboard(page, ownerRequest("/?view=library&status=unavailable"))
	if page.Code != http.StatusOK {
		t.Fatalf("unavailable library = %d", page.Code)
	}
	root, err := html.Parse(strings.NewReader(page.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	var destination *url.URL
	for node := range root.Descendants() {
		if node.Type != html.ElementNode || node.Data != "p" || subtitleHTMLAttribute(node, "class") != "subtitle-background-note" {
			continue
		}
		for link := range node.Descendants() {
			if link.Type == html.ElementNode && link.Data == "a" {
				destination, err = url.Parse(subtitleHTMLAttribute(link, "href"))
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if destination == nil || destination.Path != "/settings" || destination.Fragment == "" {
		t.Fatalf("file-access recovery lacks a Settings section: %v", destination)
	}
	settings := httptest.NewRecorder()
	showSubtitleSettings(manager.settings, manager.provider, nil)(settings, ownerRequest(destination.Path))
	if settings.Code != http.StatusOK {
		t.Fatalf("recovery destination = %d", settings.Code)
	}
	root, err = html.Parse(strings.NewReader(settings.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	for section := range root.Descendants() {
		if section.Type != html.ElementNode || section.Data != "section" || subtitleHTMLAttribute(section, "id") != destination.Fragment {
			continue
		}
		for heading := range section.Descendants() {
			if heading.Type == html.ElementNode && heading.Data == "h2" && heading.FirstChild != nil && heading.FirstChild.Data == "Media Libraries" {
				return
			}
		}
	}
	t.Fatalf("rendered recovery link %q does not resolve to Media Libraries", destination.String())
}
