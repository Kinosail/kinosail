package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/archivetest"
)

func TestViewerCanBrowseAndDownloadEbooks(t *testing.T) {
	t.Parallel()
	libraryAPIFixture.ViewerCanBrowseAndDownloadEbooks(t)
}

func TestReaderServesMainstreamComicArchivesThroughAPIAndWeb(t *testing.T) {
	t.Parallel()
	libraryAPIFixture.ReaderServesMainstreamComicArchivesThroughAPIAndWeb(t, apiItemsByTitle)
}

func TestReaderServesPlainTextBooksInBookFolders(t *testing.T) {
	t.Parallel()
	libraryAPIFixture.ReaderServesPlainTextBooksInBookFolders(t, apiItemsByTitle)
}

var writeZip = archivetest.WriteZIP
