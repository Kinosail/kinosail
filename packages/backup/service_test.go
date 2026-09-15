package backup

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRejectsInvalidConfigurationWithoutOpeningDatabase(t *testing.T) {
	opens := 0
	open := func(string) (Database, error) {
		opens++
		return nil, nil
	}
	valid := Config{"kinosail.db", open, func([]byte) error { return nil }}
	for name, mutate := range map[string]func(*Config){
		"missing filename": func(config *Config) { config.DatabaseFilename = "" },
		"nested filename":  func(config *Config) { config.DatabaseFilename = "../kinosail.db" },
		"missing opener":   func(config *Config) { config.OpenDatabase = nil },
		"missing validator": func(config *Config) {
			config.ValidateSettings = nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			config := valid
			mutate(&config)
			if service, err := New(config); err == nil || service != nil {
				t.Fatalf("invalid configuration = %#v, %v", service, err)
			}
		})
	}
	if opens != 0 {
		t.Fatalf("invalid configuration opened database %d times", opens)
	}
}

func TestWriteRejectsInvalidInputsBeforeOutput(t *testing.T) {
	service := testService(t, func(string) (Database, error) { return nil, errors.New("unexpected open") }, func([]byte) error { return nil })
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, write := range map[string]func(*bytes.Buffer) error{
		"missing version": func(output *bytes.Buffer) error { return service.Write(output, directory, "") },
		"malformed version": func(output *bytes.Buffer) error {
			return service.Write(output, directory, "v1\nunsafe")
		},
		"oversized passphrase": func(output *bytes.Buffer) error {
			return service.WriteEncrypted(output, directory, strings.Repeat("x", maxPassphraseSize+1), "dev")
		},
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			if err := write(&output); err == nil || output.Len() != 0 {
				t.Fatalf("invalid input produced %d bytes, %v", output.Len(), err)
			}
		})
	}
}

func TestWriteValidatesAllDocumentsBeforeOutput(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "api_keys.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte(`{"reject":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	service := testService(t, func(string) (Database, error) { return nil, errors.New("unexpected open") }, func(data []byte) error {
		if bytes.Contains(data, []byte("reject")) {
			return errors.New("invalid settings")
		}
		return nil
	})
	for name, write := range map[string]func(*bytes.Buffer) error{
		"plain": func(output *bytes.Buffer) error { return service.Write(output, directory, "dev") },
		"encrypted": func(output *bytes.Buffer) error {
			return service.WriteEncrypted(output, directory, "correct horse battery staple", "dev")
		},
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			if err := write(&output); err == nil || output.Len() != 0 {
				t.Fatalf("invalid final document produced %d bytes, %v", output.Len(), err)
			}
		})
	}
}

func TestWritePropagatesDatabaseFailuresBeforeOutput(t *testing.T) {
	for name, open := range map[string]func(string) (Database, error){
		"open": func(string) (Database, error) { return nil, errors.New("open failed") },
		"nil":  func(string) (Database, error) { return nil, nil },
		"export": func(string) (Database, error) {
			return &failingDatabase{exportErr: errors.New("export failed")}, nil
		},
		"close": func(string) (Database, error) {
			return &failingDatabase{documents: map[string][]byte{"settings.json": []byte(`{}`)}, closeErr: errors.New("close failed")}, nil
		},
		"semantic validation": func(string) (Database, error) {
			return &failingDatabase{documents: map[string][]byte{"settings.json": []byte(`{"reject":true}`)}}, nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "kinosail.db"), []byte("database"), 0o600); err != nil {
				t.Fatal(err)
			}
			service := testService(t, open, func(data []byte) error {
				if bytes.Contains(data, []byte("reject")) {
					return errors.New("invalid settings")
				}
				return nil
			})
			var output bytes.Buffer
			if err := service.Write(&output, directory, "dev"); err == nil || output.Len() != 0 {
				t.Fatalf("database failure produced %d bytes, %v", output.Len(), err)
			}
		})
	}
}

func testService(t *testing.T, open func(string) (Database, error), validate func([]byte) error) *Service {
	t.Helper()
	service, err := New(Config{"kinosail.db", open, validate})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type failingDatabase struct {
	documents map[string][]byte
	exportErr error
	closeErr  error
}

func (database *failingDatabase) Export() (map[string][]byte, error) {
	return database.documents, database.exportErr
}

func (database *failingDatabase) Close() error { return database.closeErr }
