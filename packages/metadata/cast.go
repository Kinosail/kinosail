package metadata

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

func validRecordCast(cast []library.Person) bool {
	if len(cast) > 15 {
		return false
	}
	for _, person := range cast {
		if strings.TrimSpace(person.Name) == "" || len(person.Name) > 200 || len(person.Role) > 200 || len(person.Image) > 4096 || hasControlText(person.Name+person.Role+person.Image) {
			return false
		}
	}
	return true
}

func applyRecordCast(item *library.Item, record Record) {
	if len(record.Cast) == 0 {
		record.Cast = record.ShowCast
	}
	if len(record.Cast) > 0 && (record.Owner || len(item.Cast) == 0) {
		item.Cast = append([]library.Person(nil), record.Cast...)
	}
	if len(record.ShowCast) > 0 && (record.Owner || len(item.ShowCast) == 0) {
		item.ShowCast = append([]library.Person(nil), record.ShowCast...)
	}
}

// Cast is optional enrichment: unavailable credits must not prevent title lookup.
func enrichTMDBCast(ctx context.Context, baseURL, kind string, id int, fetch func(context.Context, string, any) error, artwork func(string) string, result Result) (Result, error) {
	var credits struct {
		Cast []TMDBCastMember `json:"cast"`
	}
	endpoint := fmt.Sprintf("%s/%s/%d/credits", strings.TrimRight(baseURL, "/"), kind, id)
	if fetch(ctx, endpoint, &credits) != nil {
		return result, nil //nolint:nilerr // Unavailable optional credits must not prevent title lookup.
	}
	if !ValidTMDBCast(credits.Cast) {
		return Result{}, errors.New("metadata cast is invalid")
	}
	result.Record.CastFetched = true
	cast := make([]library.Person, 0, min(15, len(credits.Cast)))
	for _, actor := range credits.Cast {
		name := strings.TrimSpace(actor.Name)
		if name == "" {
			continue
		}
		person := library.Person{Name: name, Role: strings.TrimSpace(actor.Character)}
		if actor.ProfilePath != "" {
			person.Image = artwork(fmt.Sprintf("%s-%d-cast-%d", kind, id, len(cast)))
			if person.Image == "" {
				return Result{}, errors.New("metadata cast artwork is invalid")
			}
			result.Images = append(result.Images, Image{Source: actor.ProfilePath, Target: person.Image, Person: true})
		}
		cast = append(cast, person)
		if len(cast) == 15 {
			break
		}
	}
	if kind == "tv" {
		result.Record.ShowCast = cast
	} else {
		result.Record.Cast = cast
	}
	if !Bounded(result.Record) {
		return Result{}, errors.New("metadata cast is invalid")
	}
	return result, nil
}
