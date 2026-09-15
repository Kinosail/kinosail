package server

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const subtitleLibraryPageSize = 40

type subtitleDashboardOptions struct {
	View   string `json:"view"`
	Query  string `json:"query,omitempty"`
	Status string `json:"status"`
	Kind   string `json:"kind"`
	Sort   string `json:"sort"`
	Page   int    `json:"page"`
}

func subtitleDashboardQuery(request *http.Request) (subtitleDashboardOptions, error) {
	invalid := errors.New("invalid subtitle dashboard request")
	options := subtitleDashboardOptions{View: "summary", Status: "all", Kind: "all", Sort: "title", Page: 1}
	if len(request.URL.RawQuery) > 1024 {
		return options, invalid
	}
	values, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return options, invalid
	}
	for key, entries := range values {
		if len(entries) != 1 || !oneOf(key, "view", "q", "status", "kind", "sort", "page") {
			return options, invalid
		}
	}
	if value, exists := values["view"]; exists {
		options.View = value[0]
	}
	if options.View == "" {
		options.View = "summary"
	}
	options.Query = strings.TrimSpace(values.Get("q"))
	if value, exists := values["status"]; exists {
		options.Status = value[0]
	}
	if value, exists := values["kind"]; exists {
		options.Kind = value[0]
	}
	if value, exists := values["sort"]; exists {
		options.Sort = value[0]
	}
	if value, exists := values["page"]; exists {
		options.Page, err = strconv.Atoi(value[0])
		if err != nil || options.Page < 1 || options.Page > 1000000 || strconv.Itoa(options.Page) != value[0] {
			return options, invalid
		}
	}
	if !oneOf(options.View, "summary", "wanted", "library") || len(options.Query) > 128 || !utf8.ValidString(options.Query) ||
		!oneOf(options.Status, "all", "ready", "wanted", "checking", "unavailable") || !oneOf(options.Kind, "all", "movie", "episode") || !oneOf(options.Sort, "title", "modified") {
		return options, invalid
	}
	// Overview is a six-file preview; Wanted has an unambiguous status.
	if options.View == "summary" && (options.Page != 1 || options.Status != "all") || options.View == "wanted" && !oneOf(options.Status, "all", "wanted") {
		return options, invalid
	}
	return options, nil
}

func (options subtitleDashboardOptions) pageURL(page int) string {
	values := url.Values{"view": {options.View}}
	if options.Query != "" {
		values.Set("q", options.Query)
	}
	if options.Status != "all" {
		values.Set("status", options.Status)
	}
	if options.Kind != "all" {
		values.Set("kind", options.Kind)
	}
	if options.Sort != "title" {
		values.Set("sort", options.Sort)
	}
	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	return "/?" + values.Encode()
}
