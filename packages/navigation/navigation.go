// Package navigation owns the Player-canonical Library navigation model.
package navigation

import (
	"errors"
	"mime"
	"net/http"
	"slices"
	"strings"
)

const maximumFormBytes = 16 << 10

var errInvalidForm = errors.New("navigation form is invalid")

type destination struct{ id, name, href, view, group string }

// Link is one visible Library destination.
type Link struct {
	Name, Href, Group string
	Active            bool
}

// Preference is one destination in the Owner navigation editor.
type Preference struct {
	ID, Name    string
	Visible     bool
	CanMoveUp   bool
	CanMoveDown bool
}

// Controller binds the shared navigation model to app-owned state and copy.
type Controller struct {
	read     func() []string
	write    func([]string) error
	localize func(string, string) string
}

var destinations = []destination{
	{"home", "Home", "/?view=all", "all", "library"},
	{"list", "My List", "/?view=list", "list", "personal"},
	{"movies", "Movies", "/?view=movies", "movies", "library"},
	{"shows", "Shows", "/?view=shows", "shows", "library"},
	{"music", "Music", "/?view=music", "music", "library"},
	{"audiobooks", "Audiobooks", "/?view=audiobooks", "audiobooks", "library"},
	{"books", "Books", "/?view=books", "books", "library"},
	{"photos", "Photos", "/?view=photos", "photos", "library"},
	{"collections", "Collections", "/?view=collections", "collections", "personal"},
	{"playlists", "Playlists", "/?view=playlists", "playlists", "personal"},
	{"unwatched", "Unwatched", "/?view=unwatched", "unwatched", "viewing"},
	{"history", "History", "/?view=history", "history", "viewing"},
}

// NewController binds app-owned persistence and localization adapters.
func NewController(read func() []string, write func([]string) error, localize func(string, string) string) Controller {
	return Controller{read, write, localize}
}

// Default returns the complete Player navigation in its canonical order.
func Default() []string {
	items := make([]string, len(destinations))
	for index, item := range destinations {
		items[index] = item.id
	}
	return items
}

// Validate rejects empty, unknown, duplicate, or oversized navigation.
func Validate(items []string) error {
	if len(items) == 0 || len(items) > len(destinations) {
		return errors.New("navigation must contain 1 to 12 destinations")
	}
	seen := make(map[string]bool, len(items))
	for _, id := range items {
		if seen[id] || !slices.ContainsFunc(destinations, func(item destination) bool { return item.id == id }) {
			return errors.New("navigation contains an unknown or duplicate destination")
		}
		seen[id] = true
	}
	return nil
}

// Links projects the selected destinations into primary and overflow links.
func Links(items []string, view string, localize func(string) string) (primary, more []Link) {
	for _, id := range items {
		item := destinationByID(id)
		link := Link{localize(item.name), item.href, item.group, item.view == view || item.id == view}
		if len(primary) < 4 {
			primary = append(primary, link)
		} else {
			more = append(more, link)
		}
	}
	return primary, more
}

// Links projects the current destinations with app-owned localization.
func (controller Controller) Links(view, language string) (primary, more []Link) {
	return Links(controller.read(), view, func(name string) string { return controller.localize(language, name) })
}

// Preferences projects visible destinations first and then hidden destinations.
func Preferences(visible []string, localize func(string) string) []Preference {
	items := make([]Preference, 0, len(destinations))
	for index, id := range visible {
		destination := destinationByID(id)
		items = append(items, Preference{id, localize(destination.name), true, index > 0, index+1 < len(visible)})
	}
	for _, destination := range destinations {
		if !slices.Contains(visible, destination.id) {
			items = append(items, Preference{ID: destination.id, Name: localize(destination.name)})
		}
	}
	return items
}

// Preferences projects the current Owner navigation editor state.
func (controller Controller) Preferences(language string) []Preference {
	return Preferences(controller.read(), func(name string) string { return controller.localize(language, name) })
}

// Set validates and persists one navigation selection.
func (controller Controller) Set(items []string) error {
	if err := Validate(items); err != nil {
		return err
	}
	return controller.write(items)
}

func destinationByID(id string) destination {
	for _, item := range destinations {
		if item.id == id {
			return item
		}
	}
	return destination{}
}

// Move applies one validated Owner move to a detached destination list.
func Move(items, moves []string) ([]string, error) {
	if len(moves) == 0 {
		return items, nil
	}
	move, direction, found := strings.Cut(moves[0], ":")
	if len(moves) != 1 || !found || direction != "up" && direction != "down" {
		return nil, errors.New("navigation move is invalid")
	}
	position, offset := slices.Index(items, move), -1
	if direction == "down" {
		offset = 1
	}
	target := position + offset
	if position < 0 || target < 0 || target >= len(items) {
		return nil, errors.New("navigation move is invalid")
	}
	items[position], items[target] = items[target], items[position]
	return items, nil
}

// Save strictly parses and persists one Owner navigation form.
func (controller Controller) Save(request *http.Request) error {
	mediaType, _, mediaErr := mime.ParseMediaType(request.Header.Get("Content-Type"))
	request.Body = http.MaxBytesReader(nil, request.Body, maximumFormBytes)
	if mediaErr != nil {
		return errInvalidForm
	}
	if mediaType != "application/x-www-form-urlencoded" {
		return errInvalidForm
	}
	if request.URL.RawQuery != "" {
		return errInvalidForm
	}
	if request.ParseForm() != nil {
		return errInvalidForm
	}
	if !onlyFormKeys(request) {
		return errInvalidForm
	}
	items, err := Move(append([]string(nil), request.PostForm["items"]...), request.PostForm["move"])
	if err != nil {
		return err
	}
	return controller.Set(items)
}

// Handler writes errors through the app adapter and redirects successful saves.
func (controller Controller) Handler(failure func(http.ResponseWriter, *http.Request, string, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := controller.Save(request); err != nil {
			failure(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/settings#navigation", http.StatusSeeOther)
	}
}

func onlyFormKeys(request *http.Request) bool {
	for key := range request.PostForm {
		if key != "items" && key != "move" {
			return false
		}
	}
	return true
}
