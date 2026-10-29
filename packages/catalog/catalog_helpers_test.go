package catalog

import (
	"context"
	"net/url"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func BrowseLibrary(ctx context.Context, values url.Values, locale string, load func() ([]*library.Item, error), access func() BrowseAccess) (Result, error) {
	return browseLibrary(ctx, values, locale, func(Browse) (browseSource, error) { items, err := load(); return browseSource{items: items}, err }, access)
}

func NewIndex(ctx context.Context, roots []ScanRoot, cache string, interval time.Duration, acquire func(context.Context) (func(), error)) *Index {
	return NewDecoratedIndex(ctx, roots, cache, interval, acquire, Decoration{})
}

func titleLetter(title, locale string) string {
	return titleLetterWithCaser(title, cases.Upper(language.Make(locale)))
}

func SafePath(path string, allowed []string) bool {
	return resolveAllowed(allowed).contains(path)
}

func ScanRoots[T any](values []T, fields func(T) (string, string)) []ScanRoot {
	roots := make([]ScanRoot, len(values))
	for position, value := range values {
		roots[position].Path, roots[position].Namespace = fields(value)
	}
	return roots
}

func Scan(ctx context.Context, roots []ScanRoot, decorate func([]library.Item) []library.Item) ([]library.Item, error) {
	scanners := make([]*library.Scanner, len(roots))
	for position, root := range roots {
		scanners[position] = library.NewScanner(root.Path, root.Namespace)
	}
	items, err := scanRoots(ctx, scanners, func(scanner *library.Scanner) ([]library.Item, error) { return scanner.Scan(ctx) })
	if err != nil {
		return items, err
	}
	if decorate != nil {
		items = decorate(items)
	}
	return items, nil
}

func searchRank(item library.Item, query string) int {
	return searchRankNormalized(item, searchText(query))
}
