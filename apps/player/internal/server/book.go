package server

import (
	"net/http"
)

type bookDetailLists struct{ *listStore }

func (lists bookDetailLists) EditablePlaylistNames(request *http.Request) []string {
	return lists.editablePlaylistNames(request)
}

func browseBook(index *libraryIndex, lists *listStore) http.HandlerFunc {
	return detailPages.Book(index, bookDetailLists{lists})
}
