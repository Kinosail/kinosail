package catalog

import "net/url"

type Page struct {
	Offset, Limit int
	Requested     bool
}

func ParsePage(values url.Values) (Page, error) {
	offset, err := browseInteger(values, "offset", 0, 0, maximumOffset)
	if err != nil {
		return Page{}, err
	}
	limit, err := browseInteger(values, "limit", defaultPageSize, 1, maximumPageSize)
	if err != nil {
		return Page{}, err
	}
	return Page{Offset: offset, Limit: limit, Requested: values.Has("offset") || values.Has("limit")}, nil
}

func (page Page) Window(total int) (int, int) {
	start := min(page.Offset, total)
	return start, min(start+page.Limit, total)
}
