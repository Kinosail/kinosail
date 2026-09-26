package server

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

type readerPage struct {
	Title  string `json:"title"`
	URL    string `json:"url"`
	Number int    `json:"number"`
}

type readerBook struct {
	ID, Title, Type string
	Pages           []readerPage
	CurrentPage     int
	CurrentURL      string
}

const readerHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>{{.Title}} · Kinosail Player Reader</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=skeleton-3"></head><body class="detail-page"><main class="detail-shell"><a class="back" href="/book/{{.ID}}">{{icon "back"}} {{.Title}}</a><span class="eyebrow">{{.Type}} reader</span><h1>{{.Title}}</h1>{{if eq .Type "comic"}}<div class="reader-pages">{{range .Pages}}<img loading="lazy" src="{{.URL}}" alt="{{.Title}}">{{end}}</div>{{else}}<p role="status" data-reader-page="{{.CurrentPage}}">{{if eq .Type "epub"}}Chapter{{else}}Page{{end}} {{.CurrentPage}} of {{len .Pages}}</p><form class="chapter-actions" action="/read/{{.ID}}/progress" method="post" aria-label="Reading order">{{range .Pages}}<button class="mode" type="submit" name="page" value="{{.Number}}" {{if eq .Number $.CurrentPage}}aria-current="page"{{end}}>{{.Title}}</button>{{end}}</form><iframe class="book-reader" name="reader" title="{{.Title}}" src="{{.CurrentURL}}"></iframe>{{end}}</main></body></html>`

var readerView = newLocalizedTemplate("reader", readerHTML)

func registerReader(mux *http.ServeMux, index *libraryIndex, progress *progressStore) {
	mux.HandleFunc("GET /read/{id}", browseReader(index, progress))
	mux.HandleFunc("POST /read/{id}/progress", saveReaderProgress(index, progress))
	mux.HandleFunc("GET /read/{id}/file", serveReaderFile(index))
	mux.HandleFunc("GET /read/{id}/asset/{asset...}", serveReaderAsset(index))
}

func browseReader(index *libraryIndex, progress *progressStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		book, err := readerFor(request, index, request.PathValue("id"))
		if err != nil {
			localizedNotFound(writer, request)
			return
		}
		book.CurrentPage = progress.ReaderPage(request, book.ID, len(book.Pages))
		book.CurrentURL = book.Pages[book.CurrentPage-1].URL
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := readerView.Execute(writer, request, book); err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
		}
	}
}

func readerFor(request *http.Request, index *libraryIndex, id string) (readerBook, error) {
	item, found := visibleItem(request, index, id)
	if !found || item.Kind != "book" {
		return readerBook{}, errors.New("book not found")
	}
	book := readerBook{ID: item.ID, Title: item.Title, Type: strings.ToLower(item.Container)}
	switch book.Type {
	case "pdf":
		book.Pages = []readerPage{{Title: "Document", URL: "/read/" + item.ID + "/file"}}
	case "cb7", "cbt", "cbz":
		book.Type, book.Pages = "comic", archiveImages(request.Context(), item)
	case "epub":
		book.Pages = epubSpine(item)
	}
	if len(book.Pages) == 0 {
		return readerBook{}, errors.New("unsupported or empty book")
	}
	for number := range book.Pages {
		book.Pages[number].Number = number + 1
	}
	return book, nil
}

func saveReaderProgress(index *libraryIndex, progress *progressStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		book, err := readerFor(request, index, request.PathValue("id"))
		if err != nil {
			localizedNotFound(writer, request)
			return
		}
		page, parseErr := strconv.Atoi(request.FormValue("page"))
		if parseErr != nil || progress.SetReaderPage(request, book.ID, page, len(book.Pages)) != nil {
			localizedError(writer, request, "invalid reader page", http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/read/"+book.ID, http.StatusSeeOther)
	}
}

type epubManifestItem struct {
	ID   string `xml:"id,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"media-type,attr"`
}

func epubSpine(item library.Item) []readerPage { //nolint:cyclop // EPUB container, manifest, and spine validation form one parser boundary.
	archive, err := zip.OpenReader(item.Path)
	if err != nil {
		return nil
	}
	defer archive.Close()
	container, err := library.ReadArchive(archive.File, "META-INF/container.xml", 1<<20)
	if err != nil {
		return nil
	}
	var root struct {
		Rootfiles []struct {
			Path string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if xml.Unmarshal(container, &root) != nil || len(root.Rootfiles) == 0 {
		return nil
	}
	opfPath := library.CleanArchivePath(root.Rootfiles[0].Path)
	opf, err := library.ReadArchive(archive.File, opfPath, 4<<20)
	if err != nil {
		return nil
	}
	var pkg struct {
		Manifest []epubManifestItem `xml:"manifest>item"`
		Spine    []struct {
			ID string `xml:"idref,attr"`
		} `xml:"spine>itemref"`
	}
	if xml.Unmarshal(opf, &pkg) != nil {
		return nil
	}
	manifest := make(map[string]epubManifestItem, len(pkg.Manifest))
	for _, entry := range pkg.Manifest {
		manifest[entry.ID] = entry
	}
	pages := make([]readerPage, 0, len(pkg.Spine))
	for position, reference := range pkg.Spine {
		entry := manifest[reference.ID]
		name := library.CleanArchivePath(path.Join(path.Dir(opfPath), entry.Href))
		if entry.Href != "" && (entry.Type == "application/xhtml+xml" || entry.Type == "text/html") {
			pages = append(pages, readerPage{Title: "Chapter " + strconv.Itoa(position+1), URL: archiveURL(item.ID, name)})
		}
	}
	return pages
}

func archiveURL(id, name string) string {
	parts := strings.Split(library.CleanArchivePath(name), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return "/read/" + id + "/asset/" + strings.Join(parts, "/")
}

func serveReaderFile(index *libraryIndex) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		item, found := visibleItem(request, index, request.PathValue("id"))
		if !found || item.Kind != "book" || !strings.EqualFold(item.Container, "PDF") {
			localizedNotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Disposition", "inline")
		writer.Header().Set("Content-Type", "application/pdf")
		writer.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; frame-ancestors 'self'")
		writer.Header().Set("X-Frame-Options", "SAMEORIGIN")
		http.ServeFile(writer, request, item.Path)
	}
}

func serveReaderAsset(index *libraryIndex) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		item, found := visibleItem(request, index, request.PathValue("id"))
		if !found || item.Kind != "book" || library.CleanArchivePath(request.PathValue("asset")) == "" {
			localizedNotFound(writer, request)
			return
		}
		data, err := library.ReadArchiveAsset(request.Context(), item, request.PathValue("asset"))
		if err != nil {
			localizedNotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Security-Policy", "sandbox; default-src 'self' data:; script-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'self'")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "SAMEORIGIN")
		if contentType := mime.TypeByExtension(filepath.Ext(request.PathValue("asset"))); contentType != "" {
			writer.Header().Set("Content-Type", contentType)
		}
		_, _ = writer.Write(data)
	}
}

func apiReader(index *libraryIndex) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		book, err := readerFor(request, index, request.PathValue("id"))
		if err != nil {
			apiNotFound(writer)
			return
		}
		writeJSON(writer, map[string]any{"id": book.ID, "title": book.Title, "type": book.Type, "pages": book.Pages}, http.StatusOK)
	}
}

func apiReaderProgress(index *libraryIndex, progress *progressStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		includeOffset, err := readerOffsetRequested(request)
		if err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		book, err := readerFor(request, index, request.PathValue("id"))
		if err != nil {
			apiNotFound(writer)
			return
		}
		if request.Method == http.MethodGet {
			state := progress.Get(request, book.ID)
			writeJSON(writer, readerProgressResponse(state.ReaderPage, len(book.Pages), state.ReaderOffset, includeOffset), http.StatusOK)
			return
		}
		var input struct {
			Page   int             `json:"page"`
			Offset json.RawMessage `json:"offset"`
		}
		if !readJSON(writer, request, &input) {
			return
		}
		offset, err := parseReaderOffset(input.Offset)
		if err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		fraction := 0.0
		if offset != nil {
			fraction = *offset
		}
		if err := progress.SetReaderPosition(request, book.ID, input.Page, len(book.Pages), fraction); err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writeJSON(writer, readerProgressResponse(input.Page, len(book.Pages), fraction, includeOffset), http.StatusOK)
	}
}

// Omission means the start of the chapter; an explicit value must be numeric.
func parseReaderOffset(raw json.RawMessage) (*float64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value *float64
	if json.Unmarshal(raw, &value) != nil || value == nil || *value < 0 || *value > 1 {
		return nil, errors.New("invalid reading offset")
	}
	return value, nil
}

// Older native builds reject unknown response fields, so offsets are opt-in.
func readerOffsetRequested(request *http.Request) (bool, error) {
	if len(request.URL.RawQuery) > 2048 {
		return false, errors.New("invalid reader query")
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return false, errors.New("invalid reader query")
	}
	values, present := query["includeOffset"]
	if !present {
		return false, nil
	}
	if len(values) != 1 || values[0] != "true" {
		return false, errors.New("invalid reader query")
	}
	return true, nil
}
