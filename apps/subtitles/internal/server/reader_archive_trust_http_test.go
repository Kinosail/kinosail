package server_test

import (
	"bytes"
	"fmt"
	"image/color"
	"net/http"
	"testing"
)

func TestReaderRejectsDuplicateArchiveResourcesThroughHTTP(t *testing.T) { //nolint:cyclop,gocognit // Each archive order shares the same public rejection and recovery contract.
	red := readerTrustPNG(t, color.RGBA{R: 255, A: 255})
	blue := readerTrustPNG(t, color.RGBA{B: 255, A: 255})
	first := []byte("<html><body>first chapter</body></html>")
	last := []byte("<html><body>last chapter</body></html>")
	for _, format := range []string{"cbz", "cbt", "epub"} {
		for _, variant := range []string{"different", "identical", "oversized"} {
			if format == "cbt" && variant == "oversized" {
				continue // Oversized central-directory declarations are ZIP-specific.
			}
			for _, reverse := range []bool{false, true} {
				name := format + "/" + variant + "/first"
				if reverse {
					name = format + "/" + variant + "/reversed"
				}
				t.Run(name, func(t *testing.T) {
					asset := "page.png"
					bodies := [][]byte{red, blue}
					if format == "epub" {
						asset, bodies = "OEBPS/one.xhtml", [][]byte{first, last}
					}
					if variant == "identical" {
						bodies[1] = bodies[0]
					}
					entries := []readerTrustEntry{{name: asset, body: bodies[0]}, {name: asset, body: bodies[1]}}
					if variant == "oversized" {
						// The central directory declares one duplicate above the public asset cap;
						// its stored bytes stay small so this test never inflates 64MiB.
						entries[0].declaredSize = 64<<20 + 1
					}
					if reverse {
						entries[0], entries[1] = entries[1], entries[0]
					}
					if format == "epub" {
						entries = readerTrustEPUB(`<item id="one" href="one.xhtml" media-type="application/xhtml+xml"/>`, `<itemref idref="one"/>`, entries...)
					}
					handler, id, sentinel := readerTrustFixture(t, format, entries)
					readerTrustUnavailable(t, handler, id)
					readerTrustAssetRejected(t, handler, id, asset, sentinel)
				})
			}
		}
	}
}

func TestReaderRejectsDuplicateEPUBManifestIDsThroughHTTP(t *testing.T) {
	one := `<item id="chapter" href="one.xhtml" media-type="application/xhtml+xml"/>`
	two := `<item id="chapter" href="two.xhtml" media-type="application/xhtml+xml"/>`
	first := []byte("<html><body>first chapter</body></html>")
	last := []byte("<html><body>last chapter</body></html>")
	for _, fixture := range []struct{ name, declaration string }{{"first-last", one + two}, {"last-first", two + one}} {
		t.Run(fixture.name, func(t *testing.T) {
			entries := readerTrustEPUB(fixture.declaration, `<itemref idref="chapter"/>`,
				readerTrustEntry{name: "OEBPS/one.xhtml", body: first},
				readerTrustEntry{name: "OEBPS/two.xhtml", body: last})
			handler, id, _ := readerTrustFixture(t, "epub", entries)
			readerTrustUnavailable(t, handler, id)
			// The independent asset route still returns unambiguous resources by exact name.
			readerTrustAssetEquals(t, handler, id, "OEBPS/one.xhtml", first)
			readerTrustAssetEquals(t, handler, id, "OEBPS/two.xhtml", last)
		})
	}
}

func TestReaderRejectsRootEscapingEPUBSpinesThroughHTTP(t *testing.T) {
	good := `<item id="good" href="one.xhtml" media-type="application/xhtml+xml"/>`
	escape := `<item id="escape" href="../../outside.xhtml" media-type="application/xhtml+xml"/>`
	chapter := []byte("<html><body>retained valid chapter</body></html>")
	for _, fixture := range []struct{ name, manifest, spine string }{
		{"escaping-only", escape, `<itemref idref="escape"/>`},
		{"valid-then-escaping", good + escape, `<itemref idref="good"/><itemref idref="escape"/>`},
		{"escaping-then-valid", escape + good, `<itemref idref="escape"/><itemref idref="good"/>`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			entries := readerTrustEPUB(fixture.manifest, fixture.spine, readerTrustEntry{name: "OEBPS/one.xhtml", body: chapter})
			handler, id, sentinel := readerTrustFixture(t, "epub", entries)
			readerTrustUnavailable(t, handler, id)
			readerTrustAssetRejected(t, handler, id, "%2e%2e%2foutside.xhtml", sentinel)
			readerTrustAssetEquals(t, handler, id, "OEBPS/one.xhtml", chapter)
		})
	}
}

func TestReaderRejectsMalformedEPUBMetadataThroughHTTP(t *testing.T) {
	good := `<item id="good" href="one.xhtml" media-type="application/xhtml+xml"/>`
	goodRef := `<itemref idref="good"/>`
	chapter := []byte("<html><body>valid unambiguous asset</body></html>")
	for _, fixture := range []struct{ name, entry, reference, rejectedAsset string }{
		{"missing-id", `<item href="one.xhtml" media-type="application/xhtml+xml"/>`, "", ""},
		{"empty-id", `<item id="" href="one.xhtml" media-type="application/xhtml+xml"/>`, "", ""},
		{"unknown-reference", "", `<itemref idref="unknown"/>`, ""},
		{"missing-reference", "", `<itemref/>`, ""},
		{"empty-reference", "", `<itemref idref=""/>`, ""},
		{"missing-href", `<item id="bad" media-type="application/xhtml+xml"/>`, `<itemref idref="bad"/>`, ""},
		{"empty-href", `<item id="bad" href="" media-type="application/xhtml+xml"/>`, `<itemref idref="bad"/>`, ""},
		{"absolute-href", `<item id="bad" href="/outside.xhtml" media-type="application/xhtml+xml"/>`, `<itemref idref="bad"/>`, "%2foutside.xhtml"},
		{"backslash-href", `<item id="bad" href="folder\page.xhtml" media-type="application/xhtml+xml"/>`, `<itemref idref="bad"/>`, "folder%5cpage.xhtml"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			entries := readerTrustEPUB(good+fixture.entry, goodRef+fixture.reference, readerTrustEntry{name: "OEBPS/one.xhtml", body: chapter})
			handler, id, sentinel := readerTrustFixture(t, "epub", entries)
			readerTrustUnavailable(t, handler, id)
			readerTrustAssetEquals(t, handler, id, "OEBPS/one.xhtml", chapter)
			if fixture.rejectedAsset != "" {
				readerTrustAssetRejected(t, handler, id, fixture.rejectedAsset, sentinel)
			}
		})
	}
}

func TestReaderPreservesUnsupportedEPUBMediaTypeBehavior(t *testing.T) {
	chapter := []byte("<html><body>valid chapter</body></html>")
	picture := readerTrustPNG(t, color.RGBA{R: 255, A: 255})
	entries := readerTrustEPUB(`<item id="good" href="one.xhtml" media-type="application/xhtml+xml"/><item id="image" href="picture.png" media-type="image/png"/>`, `<itemref idref="good"/><itemref idref="image"/>`, readerTrustEntry{name: "OEBPS/one.xhtml", body: chapter}, readerTrustEntry{name: "OEBPS/picture.png", body: picture})
	handler, id, _ := readerTrustFixture(t, "epub", entries)
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("supported chapter plus unsupported spine media = %d %q", response.Code, response.Body.String())
	}
	var book struct {
		Pages []struct {
			URL    string
			Number int
		}
	}
	mustJSON(t, response, &book)
	if len(book.Pages) != 1 || book.Pages[0].URL != "/read/"+id+"/asset/OEBPS/one.xhtml" || book.Pages[0].Number != 1 {
		t.Errorf("unsupported spine media changed reader pages: %+v", book.Pages)
	}
	readerTrustAssetEquals(t, handler, id, "OEBPS/one.xhtml", chapter)
	readerTrustAssetEquals(t, handler, id, "OEBPS/picture.png", picture)
}

func TestReaderPreservesDistinctPagesAndRelativeEPUBChaptersThroughHTTP(t *testing.T) { //nolint:cyclop,gocognit // Valid controls verify sequential metadata and every selected resource exactly.
	red := readerTrustPNG(t, color.RGBA{R: 255, A: 255})
	blue := readerTrustPNG(t, color.RGBA{B: 255, A: 255})
	first := []byte("<html><body>first chapter</body></html>")
	last := []byte("<html><body>root chapter</body></html>")
	for _, fixture := range []struct {
		name, format, kind string
		entries            []readerTrustEntry
		assets             []readerTrustEntry
	}{
		{"distinct-comic-pages", "cbz", "comic", []readerTrustEntry{{name: "01.png", body: red}, {name: "02.png", body: blue}}, []readerTrustEntry{{name: "01.png", body: red}, {name: "02.png", body: blue}}},
		{"case-distinct-epub-chapters", "epub", "epub", readerTrustEPUB(`<item id="one" href="one.xhtml" media-type="application/xhtml+xml"/><item id="One" href="One.xhtml" media-type="application/xhtml+xml"/>`, `<itemref idref="one"/><itemref idref="One"/>`, readerTrustEntry{name: "OEBPS/one.xhtml", body: first}, readerTrustEntry{name: "OEBPS/One.xhtml", body: last}), []readerTrustEntry{{name: "OEBPS/one.xhtml", body: first}, {name: "OEBPS/One.xhtml", body: last}}},
		{"parent-relative-epub-chapter", "epub", "epub", readerTrustEPUB(`<item id="one" href="one.xhtml" media-type="application/xhtml+xml"/><item id="root" href="../root.xhtml" media-type="application/xhtml+xml"/>`, `<itemref idref="one"/><itemref idref="root"/>`, readerTrustEntry{name: "OEBPS/one.xhtml", body: first}, readerTrustEntry{name: "root.xhtml", body: last}), []readerTrustEntry{{name: "OEBPS/one.xhtml", body: first}, {name: "root.xhtml", body: last}}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			handler, id, _ := readerTrustFixture(t, fixture.format, fixture.entries)
			response := apiCall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader", nil)
			if response.Code != http.StatusOK {
				t.Fatalf("valid reader = %d %q", response.Code, response.Body.String())
			}
			var book struct {
				ID, Type string
				Pages    []struct {
					Title, URL string
					Number     int
				}
			}
			mustJSON(t, response, &book)
			if book.ID != id || book.Type != fixture.kind || len(book.Pages) != len(fixture.assets) {
				t.Fatalf("valid reader contract = %+v", book)
			}
			for index, asset := range fixture.assets {
				page := book.Pages[index]
				wantURL := "/read/" + id + "/asset/" + asset.name
				if page.Number != index+1 || page.URL != wantURL || page.Title == "" {
					t.Errorf("page %d = %+v, want sequential page at %q", index+1, page, wantURL)
				}
				readerTrustAssetEquals(t, handler, id, asset.name, asset.body)
			}
		})
	}
}

func TestReaderPreservesSameBasenamesInDistinctComicFolders(t *testing.T) { //nolint:gocognit // Each format verifies metadata, exact selected bytes, and unchanged archive bytes.
	red := readerTrustPNG(t, color.RGBA{R: 255, A: 255})
	blue := readerTrustPNG(t, color.RGBA{B: 255, A: 255})
	for _, format := range []string{"cbz", "cbt"} {
		t.Run(format, func(t *testing.T) {
			entries := []readerTrustEntry{{name: "a/page.png", body: red}, {name: "b/page.png", body: blue}}
			handler, id, _ := readerTrustFixture(t, format, entries)
			response := apiCall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader", nil)
			if response.Code != http.StatusOK {
				t.Fatalf("distinct-folder reader = %d %q", response.Code, response.Body.String())
			}
			var book struct {
				Pages []struct {
					URL    string
					Number int
				}
			}
			mustJSON(t, response, &book)
			if len(book.Pages) != 2 {
				t.Fatalf("distinct-folder pages = %+v", book.Pages)
			}
			seen := map[string]bool{}
			for index, page := range book.Pages {
				if page.Number != index+1 || seen[page.URL] {
					t.Errorf("distinct-folder page = %+v", page)
				}
				seen[page.URL] = true
			}
			for _, entry := range entries {
				if !seen["/read/"+id+"/asset/"+entry.name] {
					t.Errorf("distinct-folder resource %q absent", entry.name)
				}
				readerTrustAssetEquals(t, handler, id, entry.name, entry.body)
			}
		})
	}
}

func TestReaderRejectsCBTDuplicatesBeyondMetadataWindow(t *testing.T) { //nolint:gocognit // The boundary matrix verifies truncation, duplicate rejection, and selected asset behavior.
	red := readerTrustPNG(t, color.RGBA{R: 255, A: 255})
	blue := readerTrustPNG(t, color.RGBA{B: 255, A: 255})
	for _, duplicate := range []bool{false, true} {
		t.Run(fmt.Sprintf("duplicate=%t", duplicate), func(t *testing.T) {
			entries := []readerTrustEntry{{name: "page.png", body: red}}
			for index := range 9999 {
				entries = append(entries, readerTrustEntry{name: fmt.Sprintf("notes/%05d.txt", index)})
			}
			tail := "late.png"
			if duplicate {
				tail = "page.png"
			}
			entries = append(entries, readerTrustEntry{name: tail, body: blue})
			handler, id, sentinel := readerTrustFixture(t, "cbt", entries)
			if duplicate {
				readerTrustUnavailable(t, handler, id)
				readerTrustAssetRejected(t, handler, id, "page.png", sentinel)
				return
			}
			response := apiCall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader", nil)
			if response.Code != http.StatusOK {
				t.Fatalf("unique 10001-entry archive = %d %q", response.Code, response.Body.String())
			}
			var book struct {
				Pages []struct {
					URL    string
					Number int
				}
			}
			mustJSON(t, response, &book)
			if len(book.Pages) != 1 || book.Pages[0].URL != "/read/"+id+"/asset/page.png" || book.Pages[0].Number != 1 {
				t.Errorf("first10000 metadata window changed: %+v", book.Pages)
			}
			readerTrustAssetEquals(t, handler, id, "page.png", red)
			readerTrustAssetRejected(t, handler, id, "late.png", sentinel)
		})
	}
}

func TestReaderRejectsMalformedProgressWithoutPersisting(t *testing.T) { //nolint:cyclop,gocognit // The public write and fresh-server read prove rejected progress has no durable effect.
	for _, malformed := range []bool{false, true} {
		t.Run(fmt.Sprintf("malformed=%t", malformed), func(t *testing.T) {
			handler, id, reopen := readerTrustProgressFixture(t, malformed)
			before := readerTrustSavedProgress(t, handler, id)
			endpoint := "/api/v1/books/" + id + "/reader/progress"
			response := apiCall(t, handler, "", http.MethodPut, endpoint, map[string]any{"page": 1})
			after := readerTrustSavedProgress(t, reopen(), id)
			if malformed {
				if response.Code != http.StatusNotFound || response.Body.String() != "{\"error\":\"not found\"}\n" || response.Header().Get("Content-Type") != "application/json" {
					t.Errorf("malformed reader progress = %d %q, want canonical404", response.Code, response.Body.String())
				}
				if !bytes.Equal(before, after) {
					t.Errorf("rejected progress persisted: before=%s after=%s", before, after)
				}
				return
			}
			if response.Code != http.StatusOK || response.Body.String() != "{\"page\":1,\"total\":2}\n" {
				t.Errorf("valid progress = %d %q", response.Code, response.Body.String())
			}
			if bytes.Equal(before, after) || !bytes.Contains(after, []byte(`"readerPage":1`)) {
				t.Errorf("valid progress not persisted: %s", after)
			}
			read := apiCall(t, reopen(), "", http.MethodGet, endpoint, nil)
			if read.Code != http.StatusOK || read.Body.String() != response.Body.String() {
				t.Errorf("reopened progress = %d %q", read.Code, read.Body.String())
			}
		})
	}
}
