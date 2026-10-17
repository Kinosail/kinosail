package backup

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func (service *Service) stage(archive *tar.Reader, header *tar.Header, staged map[string][]byte, includeSecrets bool) error {
	allowed := slices.Contains(service.state.archived(includeSecrets), header.Name) || header.Name == manifestName
	maximum := service.state.maxSize(header.Name)
	if header.Typeflag != tar.TypeReg || !filepath.IsLocal(header.Name) || filepath.Base(header.Name) != header.Name || !allowed || header.Size < 0 || header.Size > int64(maximum) {
		return fmt.Errorf("backup contains invalid entry %q", header.Name)
	}
	if _, exists := staged[header.Name]; exists {
		return fmt.Errorf("backup contains duplicate entry %q", header.Name)
	}
	data, err := io.ReadAll(io.LimitReader(archive, int64(maximum+1)))
	if err != nil || int64(len(data)) != header.Size || !service.validState(header.Name, data) {
		return fmt.Errorf("backup contains invalid %s", header.Name)
	}
	staged[header.Name] = data
	return nil
}

func (service *Service) validState(name string, data []byte) bool {
	if name != "audit.jsonl" {
		return json.Valid(data) && (name != "settings.json" || service.validateSettings(data) == nil)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	if len(lines) == 1 && len(lines[0]) == 0 {
		return false
	}
	for _, line := range lines {
		if !json.Valid(line) {
			return false
		}
	}
	return true
}

func validateManifest(staged map[string][]byte) error {
	data, exists := staged[manifestName]
	if !exists {
		return nil
	}
	delete(staged, manifestName)
	var description manifest
	if httpguard.DecodeUniqueJSON(bytes.NewReader(data), maxFileSize, &description) != nil || description.Format != 1 || description.StateSchema != 1 || description.KinosailVersion == "" {
		return errors.New("backup manifest is invalid")
	}
	if _, err := time.Parse(time.RFC3339, description.CreatedAt); err != nil {
		return errors.New("backup manifest is invalid")
	}
	actual := make([]string, 0, len(staged))
	for name := range staged {
		actual = append(actual, name)
	}
	slices.Sort(actual)
	slices.Sort(description.Files)
	if !slices.Equal(actual, description.Files) || description.MediaIncluded {
		return errors.New("backup manifest does not match archive")
	}
	return nil
}
