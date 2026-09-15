package catalogapi

import (
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/unicode/norm"
)

// ActorTitle is a visible movie or deduplicated show credit.
type ActorTitle struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Year    string `json:"year,omitempty"`
	Role    string `json:"role,omitempty"`
	Artwork string `json:"artwork,omitempty"`
	URL     string `json:"url"`
}

// ActorPage contains only public links and metadata from the supplied visible library.
type ActorPage struct {
	Name   string       `json:"name"`
	Image  string       `json:"image,omitempty"`
	Movies []ActorTitle `json:"movies"`
	Shows  []ActorTitle `json:"shows"`
}

// ActorQuery rejects ambiguous or oversized transport input before a library read.
func ActorQuery(raw string) (string, error) {
	if len(raw) > 2048 {
		return "", errors.New("actor is invalid")
	}
	values, err := url.ParseQuery(raw)
	if err != nil || len(values) != 1 || len(values["name"]) != 1 {
		return "", errors.New("actor is invalid")
	}
	return actorName(values.Get("name"))
}

func actorName(name string) (string, error) {
	if !utf8.ValidString(name) || len(name) > 200 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", errors.New("actor is invalid")
	}
	name = norm.NFC.String(strings.Join(strings.Fields(name), " "))
	if name == "" || len(name) > 200 {
		return "", errors.New("actor is invalid")
	}
	return name, nil
}

// ActorLibrary matches exact normalized cast names, never titles or role substrings.
func ActorLibrary(items []library.Item, name string) (ActorPage, error) {
	name, err := actorName(name)
	if err != nil {
		return ActorPage{}, err
	}
	page := ActorPage{Name: name, Movies: []ActorTitle{}, Shows: []ActorTitle{}}
	matched := make([]library.Item, 0)
	roles := make(map[string][]string)
	for _, item := range items {
		if item.Kind != "video" {
			continue
		}
		found := page.matchCredits(item, roles)
		if found {
			matched = append(matched, item)
		}
	}
	movies, shows := library.Organize(matched)
	for _, item := range movies {
		page.Movies = append(page.Movies, ActorTitle{ID: item.ID, Title: item.Title, Year: item.Year, Role: strings.Join(roles[item.ID], " · "), Artwork: actorArtwork(item), URL: "/watch/" + item.ID})
	}
	for _, show := range shows {
		credit := showCredit(show, roles)
		page.Shows = append(page.Shows, ActorTitle{ID: show.ID, Title: show.Title, Year: show.Year, Role: strings.Join(credit, " · "), Artwork: ArtworkURL(show.ArtworkID), URL: "/show/" + show.ID})
	}
	sort.Slice(page.Movies, func(i, j int) bool {
		return page.Movies[i].Title < page.Movies[j].Title || page.Movies[i].Title == page.Movies[j].Title && page.Movies[i].ID < page.Movies[j].ID
	})
	return page, nil
}

func appendRole(roles []string, role string) []string {
	for _, existing := range roles {
		if existing == role {
			return roles
		}
	}
	return append(roles, role)
}

func actorArtwork(item library.Item) string {
	if item.Artwork == "" {
		return ""
	}
	return ArtworkURLForItem(item)
}

func (page *ActorPage) matchCredits(item library.Item, roles map[string][]string) bool {
	found := false
	for scope, cast := range [][]library.Person{item.Cast, item.ShowCast} {
		found = page.matchCast(item.ID, scope, cast, roles) || found
	}
	return found
}

func (page *ActorPage) matchCast(id string, scope int, cast []library.Person, roles map[string][]string) bool {
	found := false
	for index, person := range cast {
		candidate, err := actorName(person.Name)
		if err != nil || !strings.EqualFold(page.Name, candidate) {
			continue
		}
		found = true
		if person.Role != "" {
			roles[id] = appendRole(roles[id], person.Role)
		}
		if page.Image == "" && person.Image != "" {
			page.Image = "/person/" + id + "/" + strconv.Itoa(index)
			if scope == 1 {
				page.Image += "?scope=show"
			}
		}
	}
	return found
}

func showCredit(show library.Show, roles map[string][]string) []string {
	credit := []string{}
	for _, episode := range show.Episodes {
		for _, role := range roles[episode.ID] {
			credit = appendRole(credit, role)
		}
	}
	return credit
}
