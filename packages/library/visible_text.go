package library

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func VisibleText(value string) string {
	if strings.IndexFunc(value, hiddenText) < 0 {
		return value
	}
	return strings.Map(func(character rune) rune {
		switch {
		case character == '\t':
			return ' '
		case hiddenText(character):
			return -1
		}
		return character
	}, value)
}

func RepairedText(value string) string {
	encoded := make([]byte, 0, len(value))
	for _, character := range value {
		if character < 0 || character > 0xff {
			return VisibleText(value)
		}
		encoded = append(encoded, byte(character))
	}
	if utf8.Valid(encoded) && utf8.RuneCount(encoded) < len(encoded) {
		return VisibleText(string(encoded))
	}
	return VisibleText(value)
}

func WithVisibleText(items []Item) []Item {
	for position := range items {
		item := &items[position]
		for _, field := range []*string{
			&item.Title, &item.SortTitle, &item.Plot, &item.Tagline, &item.Genres, &item.Director, &item.Studio,
			&item.Collection, &item.Artist, &item.AlbumArtist, &item.Album, &item.ShowTitle, &item.ShowPlot, &item.ShowGenres, &item.ShowStudio,
		} {
			*field = VisibleText(*field)
		}
		item.Cast, item.ShowCast = visiblePeople(item.Cast), visiblePeople(item.ShowCast)
	}
	return items
}

func visiblePeople(people []Person) []Person {
	for position, person := range people {
		if VisibleText(person.Name) != person.Name || VisibleText(person.Role) != person.Role {
			people = append([]Person(nil), people...)
			for index := position; index < len(people); index++ {
				people[index].Name, people[index].Role = VisibleText(people[index].Name), VisibleText(people[index].Role)
			}
			return people
		}
	}
	return people
}

func hiddenText(character rune) bool {
	return character != '\n' && character != '\r' && (unicode.IsControl(character) || unicode.In(character, unicode.Cf))
}
