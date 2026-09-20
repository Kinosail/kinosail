package updatecontrol

import (
	"encoding/hex"
	"errors"
	"io"
	"slices"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const (
	ManifestSchemaVersion             = 2
	InstallationContractSchemaVersion = 3
	manifestLimit                     = 64 << 10
)

type Manifest struct {
	SchemaVersion              int                  `json:"schemaVersion"`
	Version                    string               `json:"version"`
	UpdateSchema               int                  `json:"updateSchema"`
	StateSchema                int                  `json:"stateSchema"`
	MinimumStateSchema         int                  `json:"minimumStateSchema"`
	ConfigurationSchema        int                  `json:"configurationSchema"`
	MinimumConfigurationSchema int                  `json:"minimumConfigurationSchema"`
	RuntimeCommands            []string             `json:"runtimeCommands"`
	Installation               InstallationContract `json:"installation"`
	Artifacts                  []Artifact           `json:"artifacts"`
}

type InstallationContract struct {
	SchemaVersion int    `json:"schemaVersion"`
	File          string `json:"file"`
	SHA256        string `json:"sha256"`
	Size          int64  `json:"size"`
}

type Artifact struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	File   string `json:"file"`
	Format string `json:"format"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func ParseManifest(input io.Reader) (Manifest, error) {
	var manifest Manifest
	if httpguard.DecodeUniqueJSON(input, manifestLimit, &manifest) != nil || validateManifest(manifest) != nil {
		return Manifest{}, errors.New("invalid release manifest")
	}
	return manifest, nil
}

func (manifest Manifest) Select(goos, arch string) (Artifact, error) {
	if goos == "darwin" {
		goos = "macos"
	}
	for _, artifact := range manifest.Artifacts {
		if artifact.OS == goos && artifact.Arch == arch {
			return artifact, nil
		}
	}
	return Artifact{}, errors.New("release does not support this platform")
}

func (manifest Manifest) Compatible(stateSchema, configurationSchema int) error {
	if stateSchema < manifest.MinimumStateSchema || stateSchema > manifest.StateSchema || configurationSchema < manifest.MinimumConfigurationSchema || configurationSchema > manifest.ConfigurationSchema {
		return errors.New("release is incompatible with this installation")
	}
	return nil
}

func validateManifest(manifest Manifest) error { //nolint:cyclop // Manifest trust-boundary checks remain explicit and fail-closed.
	if manifest.SchemaVersion != ManifestSchemaVersion || manifest.UpdateSchema != PlanSchemaVersion || !ValidReleaseVersion(manifest.Version) || !validSchemaRange(manifest.MinimumStateSchema, manifest.StateSchema) || !validSchemaRange(manifest.MinimumConfigurationSchema, manifest.ConfigurationSchema) || !slices.Equal(manifest.RuntimeCommands, []string{"ffmpeg", "ffprobe", "fpcalc"}) || !validInstallationContract(manifest.Installation) || len(manifest.Artifacts) != 6 {
		return errors.New("invalid release manifest")
	}
	seen := make(map[string]bool, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		key := artifact.OS + "/" + artifact.Arch
		extension := ".tar.gz"
		if artifact.OS == "windows" {
			extension = ".zip"
		}
		if !slices.Contains([]string{"linux", "macos", "windows"}, artifact.OS) || !slices.Contains([]string{"amd64", "arm64"}, artifact.Arch) || seen[key] || artifact.Format != extension[1:] || artifact.File != "kinosail-core-"+manifest.Version+"-"+artifact.OS+"-"+artifact.Arch+extension || artifact.Size < 1 || artifact.Size > 512<<20 || len(artifact.SHA256) != 64 || !lowerHex(artifact.SHA256) {
			return errors.New("invalid release artifact")
		}
		seen[key] = true
	}
	return nil
}

func validInstallationContract(contract InstallationContract) bool {
	return contract.SchemaVersion == InstallationContractSchemaVersion && contract.File == "kinosail-native-installation.json" && contract.Size >= 1 && contract.Size <= manifestLimit && len(contract.SHA256) == 64 && lowerHex(contract.SHA256)
}

func validSchemaRange(minimum, current int) bool {
	return minimum >= 1 && minimum <= current && current <= 1000
}

func lowerHex(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}
