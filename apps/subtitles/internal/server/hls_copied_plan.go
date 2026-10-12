package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/hlsmanifest"
)

var errCopiedHLSPlan = errors.New("copied HLS generation plan is invalid")

const copiedHLSPlanFile = ".copied-cuts"

type copiedHLSPlan struct {
	Version  int
	Policy   string
	Strategy string
	Cuts     []float64
}

func copiedHLSMode(recipe hlsRecipe) bool {
	return recipe.mode == "remux" || recipe.mode == "audio-transcode"
}

func readHLSGenerationFile(root *os.Root, name string, maximum int64) ([]byte, error) {
	linked, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !validHLSGenerationFile(linked, name, maximum) {
		return nil, errCopiedHLSPlan
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(linked, opened) || !validHLSGenerationFile(opened, name, maximum) {
		return nil, errCopiedHLSPlan
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return nil, errCopiedHLSPlan
	}
	return data, nil
}

func loadCopiedHLSPlan(root *os.Root, policy string) (*copiedHLSPlan, error) {
	binding, err := readHLSGenerationFile(root, ".source", 16<<10)
	if err != nil || string(binding) != policy {
		return nil, errCopiedHLSPlan
	}
	data, err := readHLSGenerationFile(root, copiedHLSPlanFile, 128<<10)
	if err != nil {
		return nil, errCopiedHLSPlan
	}
	var plan copiedHLSPlan
	if json.Unmarshal(data, &plan) != nil || !plan.valid(policy) {
		return nil, errCopiedHLSPlan
	}
	canonical, err := json.Marshal(plan)
	if err != nil || !bytes.Equal(data, canonical) {
		return nil, errCopiedHLSPlan
	}
	if !copiedHLSPlanBound(root, data) {
		return nil, errCopiedHLSPlan
	}
	return &plan, nil
}

func (plan *copiedHLSPlan) valid(policy string) bool {
	if plan.Version != 1 || plan.Policy != policy || len(policy) == 0 || len(policy) > 16<<10 {
		return false
	}
	if plan.Strategy == "legacy" {
		return len(plan.Cuts) == 0
	}
	if plan.Strategy != "certified" || len(plan.Cuts) < 2 || len(plan.Cuts) > maxHLSPlaylistURIs || plan.Cuts[0] != 0 {
		return false
	}
	return validCopiedHLSCuts(plan.Cuts)
}

func validCopiedHLSCuts(cuts []float64) bool {
	for number, point := range cuts {
		if math.IsNaN(point) || math.IsInf(point, 0) || point < 0 || point > 7*24*60*60 {
			return false
		}
		if number > 0 && (point <= cuts[number-1] || point-cuts[number-1] > 60) {
			return false
		}
	}
	return true
}

func (plan *copiedHLSPlan) project(manifest []byte, duration float64) ([]byte, bool, error) {
	if plan == nil || plan.Strategy == "legacy" || bytes.Contains(manifest, []byte("#EXT-X-STREAM-INF:")) {
		projected, ready := hlsmanifest.CompleteVOD(manifest, duration, 4)
		return projected, ready, nil
	}
	projected, ready := hlsmanifest.CompleteCuts(manifest, plan.Cuts)
	if !ready {
		return nil, false, errCopiedHLSPlan
	}
	return projected, true, nil
}

func (plan *copiedHLSPlan) validateRendition(root *os.Root, name string) error {
	if plan == nil || plan.Strategy != "certified" || name == "index.m3u8" {
		return nil
	}
	manifest, err := readHLSGenerationFile(root, filepath.Join(filepath.Dir(name), "index.m3u8"), maxHLSPlaylistBytes)
	if err != nil {
		return err
	}
	if !hlsmanifest.MatchesCuts(manifest, plan.Cuts) {
		return errCopiedHLSPlan
	}
	return nil
}

func validateCopiedHLSCache(directory, policy string) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	plan, err := loadCopiedHLSPlan(root, policy)
	if err != nil {
		return err
	}
	master, err := readHLSGenerationFile(root, "index.m3u8", maxHLSPlaylistBytes)
	if err != nil {
		return err
	}
	for _, name := range strings.Split(string(master), "\n") {
		if name != "" && !strings.HasPrefix(name, "#") {
			if !validHLSManifestURI(name) {
				return errCopiedHLSPlan
			}
			if err := plan.validateRendition(root, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func copiedHLSPlanBinding(data []byte) []byte {
	return fmt.Appendf(nil, "#KINOSAIL-COPIED-CUTS:%x\n", sha256.Sum256(data))
}

func (plan *copiedHLSPlan) bindMaster(path string, manifest []byte) error {
	if plan != nil {
		data, err := json.Marshal(plan)
		if err != nil {
			return err
		}
		manifest = append(manifest, copiedHLSPlanBinding(data)...)
	}
	return writeAtomicFile(path, manifest)
}

func validHLSGenerationFile(info os.FileInfo, name string, maximum int64) bool {
	return info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= maximum &&
		(!strings.HasPrefix(name, ".") || info.Mode().Perm()&0o077 == 0)
}

func copiedHLSPlanBound(root *os.Root, data []byte) bool {
	master, err := readHLSGenerationFile(root, "index.m3u8", maxHLSPlaylistBytes)
	return err == nil && bytes.Count(master, []byte("#KINOSAIL-COPIED-CUTS:")) == 1 &&
		bytes.Contains(master, append([]byte{'\n'}, copiedHLSPlanBinding(data)...))
}
