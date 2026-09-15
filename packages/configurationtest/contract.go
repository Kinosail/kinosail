// Package configurationtest runs common regression scenarios against an app's configuration implementation.
package configurationtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type publicProjection[Source ~string] struct {
	Key        string `json:"key"`
	Env        string `json:"env"`
	Value      string `json:"value,omitempty"`
	Source     Source `json:"source"`
	Secret     bool   `json:"secret"`
	Restart    bool   `json:"restartRequired"`
	Configured bool   `json:"configured"`
}
type publicValue[Source ~string] interface {
	~struct {
		Key        string `json:"key"`
		Env        string `json:"env"`
		Value      string `json:"value,omitempty"`
		Source     Source `json:"source"`
		Secret     bool   `json:"secret"`
		Restart    bool   `json:"restartRequired"`
		Configured bool   `json:"configured"`
	}
}
type snapshot[Self any, Value any, Source ~string] interface {
	String(string) string
	Bool(string) bool
	Int(string) int
	Duration(string) time.Duration
	Strings(string) []string
	Source(string) Source
	Managed(string) bool
	Public(string) Value
	Fields() []Value
	Clone() Self
}
type contract[Source ~string, Value publicValue[Source], Snapshot snapshot[Snapshot, Value, Source]] struct {
	Load                            func(string, string, func(string) (string, bool)) (Snapshot, error)
	Set                             func(string, string, string) error
	Delete                          func(string, string) error
	TrustedOrigin                   func(string, string) (string, error)
	UpdateGUI                       func(*Snapshot, string, string, bool)
	Default, GUI, YAML, Environment Source
	Port                            string
}

// Run executes the full common suite against the supplied app operations.
func Run[Source ~string, Value publicValue[Source], Snapshot snapshot[Snapshot, Value, Source]](
	t *testing.T,
	load func(string, string, func(string) (string, bool)) (Snapshot, error),
	set func(string, string, string) error,
	deleteValue func(string, string) error,
	trustedOrigin func(string, string) (string, error),
	update func(*Snapshot, string, string, bool),
	defaultSource, gui, yaml, environment Source,
	port string,
) {
	contract := contract[Source, Value, Snapshot]{load, set, deleteValue, trustedOrigin, update, defaultSource, gui, yaml, environment, port}
	t.Run("LoadPrecedenceSourcesAndSecretFiles", contract.testLoadPrecedenceSourcesAndSecretFiles)
	t.Run("LoadRejectsUnknownAndInvalidValues", contract.testLoadRejectsUnknownAndInvalidValues)
	t.Run("TLSHostsAreStrictlyBoundedHostnamesAndAddresses", contract.testTLSHostsAreStrictlyBoundedHostnamesAndAddresses)
	t.Run("TrustedHTTPSConfigurationIsSecretAndCanonical", contract.testTrustedHTTPSConfigurationIsSecretAndCanonical)
	t.Run("TrustedHTTPSConfigurationRejectsCrossFieldConflicts", contract.testTrustedHTTPSConfigurationRejectsCrossFieldConflicts)
	t.Run("JellyfinCompatibilityRequiresTrustedHTTPSAcrossConfigurationSources", contract.testJellyfinCompatibilityRequiresTrustedHTTPSAcrossConfigurationSources)
	t.Run("SecureRemoteAccessConfigurationIsCompleteAndStrict", contract.testSecureRemoteAccessConfigurationIsCompleteAndStrict)
	t.Run("SupporterConfigurationRequiresSafeHTTPSURLs", contract.testSupporterConfigurationRequiresSafeHTTPSURLs)
	t.Run("SetPersistsSecretsSeparatelyAndHonorsManagedFields", contract.testSetPersistsSecretsSeparatelyAndHonorsManagedFields)
	t.Run("ExampleYAMLLoads", contract.testExampleYAMLLoads)
	t.Run("ContainerDefaultsRemainGUIEditable", contract.testContainerDefaultsRemainGUIEditable)
	t.Run("YAMLDataDirectoryBootstrapsGUIState", contract.testYAMLDataDirectoryBootstrapsGUIState)
	t.Run("TranscodingAcceleratorValidationIsSharedAcrossConfigurationSources", contract.testTranscodingAcceleratorValidationIsSharedAcrossConfigurationSources)
	t.Run("TranscodingCodecValidationIsSharedAcrossConfigurationSources", contract.testTranscodingCodecValidationIsSharedAcrossConfigurationSources)
	t.Run("OwnerCanInspectUpdateAndDeleteGUIConfiguration", contract.testOwnerCanInspectUpdateAndDeleteGUIConfiguration)
	t.Run("ConfigurationRejectsInvalidStoredAndExternalValues", contract.testConfigurationRejectsInvalidStoredAndExternalValues)
	t.Run("ExternalConfigurationSupportsSecretFiles", contract.testExternalConfigurationSupportsSecretFiles)
	t.Run("YAMLUsesNativeTypes", contract.testYAMLUsesNativeTypes)
}

// NoEnvironment supplies an empty environment to configuration tests.
func NoEnvironment(string) (string, bool) { return "", false }

// Lookup exposes a test environment map.
func Lookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

// WriteConfigFile creates a private fixture in a new test directory.
func WriteConfigFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	WriteAt(t, path, contents)
	return path
}

// WriteAt writes a private configuration fixture.
func WriteAt(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
