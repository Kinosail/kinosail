package library_test

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

type (
	literalArchiveEntry struct{ name, body string }
	literalArchiveCase  struct{ name, literal, other string }
	literalArchiveOrder struct {
		name          string
		first, second literalArchiveEntry
	}
)

func TestReadArchiveAssetPreservesLiteralCBTNames(t *testing.T) {
	runLiteralArchiveMatrix(t, literalArchiveCases(), writeLiteralArchive, [4]string{
		"literal named resource\n",
		"distinct other resource\n",
		"ordinary exact-name control\n",
		"leading-dash exact-name control\n",
	})
}

func literalArchiveCases() []literalArchiveCase {
	return []literalArchiveCase{
		{"star", "page*.png", "page1.png"},
		{"question", "page?.png", "page1.png"},
		{"bracket-class", "page[ab].png", "pagea.png"},
		{"opening-bracket", "page[.png", "page1.png"},
		{"closing-bracket", "page].png", "page1.png"},
		{"nested-star", "folder*/page.png", "folder1/page.png"},
		{"nested-question", "folder?/page.png", "folder1/page.png"},
		{"nested-bracket-class", "folder[ab]/page.png", "foldera/page.png"},
		{"nested-opening-bracket", "folder[/page.png", "folder1/page.png"},
		{"nested-closing-bracket", "folder]/page.png", "folder1/page.png"},
		{"combined-pattern-characters", "folder[a*?]/page[a*?].png", "foldera/pagea.png"},
		{"leading-caret", "^folder/page.png", "folder/page.png"},
		{"terminal-dollar", "page.png$", "page.png"},
		{"recursive-prefix", "page.png", "page.png/extra.png"},
	}
}

func TestReadArchiveAssetPreservesLiteralCB7Names(t *testing.T) {
	runLiteralArchiveMatrix(t, []literalArchiveCase{
		{"star", "page*.png", "page1.png"},
		{"leading-caret", "^folder/page.png", "folder/page.png"},
		{"terminal-dollar", "page.png$", "page.png"},
		{"recursive-prefix", "page.png", "page.png/extra.png"},
	}, writeLiteralCB7Archive, [4]string{
		"literal named CB7 resource\n",
		"distinct other CB7 resource\n",
		"ordinary CB7 exact-name control\n",
		"leading-dash CB7 exact-name control\n",
	})
}

func runLiteralArchiveMatrix(t *testing.T, cases []literalArchiveCase, factory func(*testing.T, []literalArchiveEntry) library.Item, bodies [4]string) {
	t.Helper()
	if _, err := exec.LookPath("bsdtar"); err != nil {
		t.Fatalf("bsdtar is required: %v", err)
	}
	for _, fixture := range cases {
		for _, order := range []literalArchiveOrder{
			{"first", literalArchiveEntry{fixture.literal, bodies[0]}, literalArchiveEntry{fixture.other, bodies[1]}},
			{"reversed", literalArchiveEntry{fixture.other, bodies[1]}, literalArchiveEntry{fixture.literal, bodies[0]}},
		} {
			t.Run(fixture.name+"/"+order.name, func(t *testing.T) {
				entries := []literalArchiveEntry{
					order.first,
					order.second,
					{"plain.png", bodies[2]},
					{"-dash.png", bodies[3]},
				}
				item := factory(t, entries)
				for _, entry := range entries {
					data, err := library.ReadArchiveAsset(t.Context(), item, entry.name)
					if err != nil || !bytes.Equal(data, []byte(entry.body)) {
						t.Errorf("literal asset %q = %q, %v; want only %q", entry.name, data, err, entry.body)
					}
				}
			})
		}
	}
}

func writeLiteralArchive(t *testing.T, entries []literalArchiveEntry) library.Item {
	t.Helper()
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	for _, entry := range entries {
		if err := writer.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o600, Size: int64(len(entry.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "Literal.cbt")
	if err := os.WriteFile(file, output.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	preserveLiteralArchive(t, file, output.Bytes())
	return library.Item{Path: file, Container: "cBt"}
}

func writeLiteralCB7Archive(t *testing.T, entries []literalArchiveEntry) library.Item {
	t.Helper()
	input := writeLiteralArchive(t, entries)
	file := filepath.Join(t.TempDir(), "Literal.cb7")
	command := exec.CommandContext(t.Context(), "bsdtar", "-cf", file, "--format=7zip", "@"+input.Path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("installed bsdtar 7z fixture generation failed: %v: %s", err, output)
	}
	archive, err := os.ReadFile(file)
	if err != nil || !bytes.HasPrefix(archive, []byte{'7', 'z', 0xbc, 0xaf, 0x27, 0x1c}) {
		t.Fatalf("fixture is not a generated 7z archive: %v", err)
	}
	preserveLiteralArchive(t, file, archive)
	return library.Item{Path: file, Container: "cB7"}
}

func preserveLiteralArchive(t *testing.T, file string, archive []byte) {
	t.Helper()
	before := sha256.Sum256(archive)
	t.Cleanup(func() {
		after, err := os.ReadFile(file)
		if err != nil || sha256.Sum256(after) != before {
			t.Errorf("literal asset read changed source archive %s: %v", file, err)
		}
		files, err := os.ReadDir(filepath.Dir(file))
		if err != nil || len(files) != 1 || files[0].Name() != filepath.Base(file) {
			t.Errorf("literal asset read extracted files: %v, %v", files, err)
		}
	})
}

func TestReadArchiveAssetRejectsExistingBackslashCBTMember(t *testing.T) {
	if _, err := exec.LookPath("bsdtar"); err != nil {
		t.Fatalf("bsdtar is required: %v", err)
	}
	item := writeLiteralArchive(t, []literalArchiveEntry{
		{`folder\page.png`, "existing rejected backslash member\n"},
		{"folder/page.png", "existing accepted slash member\n"},
	})
	data, err := library.ReadArchiveAsset(t.Context(), item, `folder\page.png`)
	if err == nil || data != nil || err.Error() != "archive entry not found" {
		t.Errorf("existing backslash asset = %q, %v; want rejected before extraction", data, err)
	}
	control, err := library.ReadArchiveAsset(t.Context(), item, "folder/page.png")
	if err != nil || string(control) != "existing accepted slash member\n" {
		t.Errorf("slash asset control = %q, %v; want exact admitted resource", control, err)
	}
}
