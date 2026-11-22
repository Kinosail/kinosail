package servertest

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var templateMessage = regexp.MustCompile(`\{\{tf? ("(?:[^"\\]|\\.)*")[ }]`)

func TemplateMessagesAreCatalogued(t *testing.T, roots []string, english map[string]string) {
	t.Helper()
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, string(filepath.Separator)+"servertest"+string(filepath.Separator)) {
				return err
			}
			source, err := os.ReadFile(path) //nolint:gosec // Paths come from the repository source tree under test.
			if err != nil {
				return err
			}
			for _, match := range templateMessage.FindAllStringSubmatch(string(source), -1) {
				message, err := strconv.Unquote(match[1])
				if err != nil || strings.Contains(message, "` + ") {
					continue
				}
				if _, ok := english[message]; !ok {
					t.Errorf("%s: %q is not in the English catalog", path, message)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
