package configurationtest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func (configuration contract[Source, PublicValue, Snapshot]) testOwnerCanInspectUpdateAndDeleteGUIConfiguration(t *testing.T) { //nolint:cyclop // One cross-layer contract deliberately sequences the complete lifecycle; exact policy remains below 22.
	directory := t.TempDir()
	loaded, err := configuration.Load(directory, "", NoEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	configuration.assertSnapshotDefaultsAndClone(t, loaded)
	configuration.assertLiveGUIUpdateAndDelete(t)

	for key, value := range map[string]string{"server.name": "Family Server", "integrations.tmdb.token": "secret"} {
		if err := configuration.Set(directory, key, value); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"server.name", "integrations.tmdb.token"} {
		if err := configuration.Delete(directory, key); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err = configuration.Load(directory, "", NoEnvironment)
	if err != nil || loaded.String("server.name") != "" || loaded.String("integrations.tmdb.token") != "" {
		t.Fatalf("deleted values remained: name=%q token=%q err=%v", loaded.String("server.name"), loaded.String("integrations.tmdb.token"), err)
	}
	if err := configuration.Set(directory, "unknown", "value"); err == nil {
		t.Fatal("unknown setting was saved")
	}
	if err := configuration.Set(directory, "backup.retention", "zero"); err == nil {
		t.Fatal("invalid setting was saved")
	}
	if err := configuration.Delete(directory, "unknown"); err == nil {
		t.Fatal("unknown setting was deleted")
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testConfigurationRejectsInvalidStoredAndExternalValues(t *testing.T) { //nolint:cyclop,funlen // The table documents independent public validation cases.
	for name, setup := range map[string]func(*testing.T) (string, string, map[string]string){
		"missing YAML": func(t *testing.T) (string, string, map[string]string) {
			return t.TempDir(), filepath.Join(t.TempDir(), "missing.yaml"), nil
		},
		"malformed YAML": func(t *testing.T) (string, string, map[string]string) {
			path := WriteConfigFile(t, "kinosail.yaml", "version: [")
			return t.TempDir(), path, nil
		},
		"oversized YAML": func(t *testing.T) (string, string, map[string]string) {
			path := WriteConfigFile(t, "kinosail.yaml", "version: 1\n#"+strings.Repeat("x", (1<<20)+1))
			return t.TempDir(), path, nil
		},
		"wrong YAML scalar type": func(t *testing.T) (string, string, map[string]string) {
			path := WriteConfigFile(t, "kinosail.yaml", "version: 1\nserver:\n  name: true\n")
			return t.TempDir(), path, nil
		},
		"ambiguous YAML key": func(t *testing.T) (string, string, map[string]string) {
			path := WriteConfigFile(t, "kinosail.yaml", "version: 1\nserver:\n  name: Nested\nserver.name: Flat\n")
			return t.TempDir(), path, nil
		},
		"malformed GUI JSON": func(t *testing.T) (string, string, map[string]string) {
			directory := t.TempDir()
			WriteAt(t, filepath.Join(directory, "configuration.json"), "{")
			return directory, "", nil
		},
		"unknown GUI key": func(t *testing.T) (string, string, map[string]string) {
			directory := t.TempDir()
			WriteAt(t, filepath.Join(directory, "configuration.json"), `{"unknown":"value"}`)
			return directory, "", nil
		},
		"secret in regular GUI file": func(t *testing.T) (string, string, map[string]string) {
			directory := t.TempDir()
			WriteAt(t, filepath.Join(directory, "configuration.json"), `{"integrations.tmdb.token":"secret"}`)
			return directory, "", nil
		},
		"invalid saved value": func(t *testing.T) (string, string, map[string]string) {
			directory := t.TempDir()
			WriteAt(t, filepath.Join(directory, "configuration.json"), `{"backup.retention":"0"}`)
			return directory, "", nil
		},
		"unknown YAML secret file": func(t *testing.T) (string, string, map[string]string) {
			path := WriteConfigFile(t, "kinosail.yaml", "version: 1\nserver:\n  name_file: value\n")
			return t.TempDir(), path, nil
		},
		"missing YAML secret file": func(t *testing.T) (string, string, map[string]string) {
			path := WriteConfigFile(t, "kinosail.yaml", "version: 1\nintegrations:\n  tmdb:\n    token_file: missing\n")
			return t.TempDir(), path, nil
		},
		"direct and file environment secret": func(t *testing.T) (string, string, map[string]string) {
			return t.TempDir(), "", map[string]string{"KINOSAIL_TMDB_TOKEN": "direct", "KINOSAIL_TMDB_TOKEN_FILE": "file"}
		},
		"file environment non-secret": func(t *testing.T) (string, string, map[string]string) {
			return t.TempDir(), "", map[string]string{"KINOSAIL_SERVER_NAME_FILE": "file"}
		},
		"missing environment secret file": func(t *testing.T) (string, string, map[string]string) {
			return t.TempDir(), "", map[string]string{"KINOSAIL_TMDB_TOKEN_FILE": filepath.Join(t.TempDir(), "missing")}
		},
		"oversized environment secret file": func(t *testing.T) (string, string, map[string]string) {
			path := WriteConfigFile(t, "secret", string(make([]byte, (16<<10)+1)))
			return t.TempDir(), "", map[string]string{"KINOSAIL_TMDB_TOKEN_FILE": path}
		},
		"public environment secret file": func(t *testing.T) (string, string, map[string]string) {
			path := WriteConfigFile(t, "secret", "exposed")
			if err := os.Chmod(path, 0o644); err != nil { //nolint:gosec // The boundary must reject deliberately public secret material.
				t.Fatal(err)
			}
			return t.TempDir(), "", map[string]string{"KINOSAIL_TMDB_TOKEN_FILE": path}
		},
		"invalid environment value": func(t *testing.T) (string, string, map[string]string) {
			return t.TempDir(), "", map[string]string{"KINOSAIL_BACKUP_RETENTION": "0"}
		},
		"DLNA without URL": func(t *testing.T) (string, string, map[string]string) {
			return t.TempDir(), "", map[string]string{"KINOSAIL_DLNA_ENABLED": "true"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			dataDir, yamlPath, environment := setup(t)
			if _, err := configuration.Load(dataDir, yamlPath, Lookup(environment)); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testExternalConfigurationSupportsSecretFiles(t *testing.T) {
	directory := t.TempDir()
	WriteAt(t, filepath.Join(directory, "token"), "yaml-secret\n")
	yamlPath := filepath.Join(directory, "kinosail.yaml")
	WriteAt(t, yamlPath, "version: 1\nintegrations:\n  tmdb:\n    token_file: token\n")
	loaded, err := configuration.Load(t.TempDir(), yamlPath, NoEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.String("integrations.tmdb.token") != "yaml-secret" || !loaded.Managed("integrations.tmdb.token") {
		t.Fatalf("external configuration = token %q", loaded.String("integrations.tmdb.token"))
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testYAMLUsesNativeTypes(t *testing.T) {
	path := WriteConfigFile(t, "kinosail.yaml", "version: 1\nplayback:\n  autoplay: true\nbackup:\n  interval: 12h\n  retention: 9\nlibraries: [Movies, Shows]\n")
	loaded, err := configuration.Load(t.TempDir(), path, NoEnvironment)
	if err != nil || !loaded.Bool("playback.autoplay") || loaded.Duration("backup.interval") != 12*time.Hour || loaded.Int("backup.retention") != 9 || !slices.Equal(loaded.Strings("libraries"), []string{"Movies", "Shows"}) {
		t.Fatalf("typed YAML = autoplay %v interval %v retention %d libraries %#v error %v", loaded.Bool("playback.autoplay"), loaded.Duration("backup.interval"), loaded.Int("backup.retention"), loaded.Strings("libraries"), err)
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) assertSnapshotDefaultsAndClone(t *testing.T, loaded Snapshot) {
	t.Helper()
	if loaded.Duration("scanning.interval") != 10*time.Minute || loaded.Managed("server.name") || publicProjection[Source](loaded.Public("listen")).Env != "KINOSAIL_LISTEN" {
		t.Fatalf("unexpected defaults: interval=%v managed=%v listen=%#v", loaded.Duration("scanning.interval"), loaded.Managed("server.name"), loaded.Public("listen"))
	}
	cloned := loaded.Clone()
	configuration.UpdateGUI(&cloned, "server.name", "Clone", false)
	if loaded.String("server.name") == cloned.String("server.name") {
		t.Fatal("cloned configuration changed its source snapshot")
	}
	fields := loaded.Fields()
	if len(fields) < 40 || !slices.IsSortedFunc(fields, func(left, right PublicValue) int {
		if publicProjection[Source](left).Key < publicProjection[Source](right).Key {
			return -1
		}
		if publicProjection[Source](left).Key > publicProjection[Source](right).Key {
			return 1
		}
		return 0
	}) {
		t.Fatal("public configuration fields were incomplete or unsorted")
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) assertLiveGUIUpdateAndDelete(t *testing.T) {
	t.Helper()
	var live Snapshot
	configuration.UpdateGUI(&live, "server.name", "Living Room", false)
	if live.String("server.name") != "Living Room" || live.Source("server.name") != configuration.GUI {
		t.Fatalf("updated snapshot = %#v", live.Public("server.name"))
	}
	configuration.UpdateGUI(&live, "server.name", "", true)
	if live.String("server.name") != "" || live.Source("server.name") != configuration.Default {
		t.Fatalf("deleted snapshot = %#v", live.Public("server.name"))
	}
}
