package configurationcore

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

var errPairBlocked = errors.New("blocked")

func validPair(dependencies *pairDependencies) Pair {
	return Pair{
		RegularFile: "configuration.json",
		SecretsFile: "secrets.json",
		Transaction: Transaction{
			RegularFile: "configuration.json",
			SecretsFile: "secrets.json",
			Parse: func([]byte, string) (map[string]string, error) {
				return map[string]string{}, nil
			},
			Validate: func(map[string]string, map[string]string) error { return nil },
		},
		dependencies: dependencies,
	}
}

func workingPairDependencies() *pairDependencies {
	stage := 0
	return &pairDependencies{
		mkdirAll: func(string, os.FileMode) error { return nil },
		createTemp: func(string, string) (StagedFile, error) {
			stage++
			return &stagedFileStub{name: filepath.Join("stages", strconv.Itoa(stage))}, nil
		},
		readFile:          func(string) ([]byte, error) { return nil, os.ErrNotExist },
		rename:            func(string, string) error { return nil },
		writeFile:         func(string, []byte, os.FileMode) error { return nil },
		remove:            func(string) error { return nil },
		writeTransaction:  func(string, []byte, []byte) error { return nil },
		removeTransaction: func(string) error { return nil },
	}
}

func TestPairPersistsThroughDefaultFilesystem(t *testing.T) {
	pair := validPair(nil)
	directory := t.TempDir()
	if err := pair.Persist(directory, map[string]string{"name": "regular"}, map[string]string{"token": "secret"}, true, true); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"configuration.json": `{"name":"regular"}`, "secrets.json": `{"token":"secret"}`} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, %v", name, data, err)
		}
	}
	if err := pair.Persist(directory, map[string]string{"name": "changed"}, nil, true, false); err != nil {
		t.Fatal(err)
	}
	if err := (Pair{}).Persist("", nil, nil, false, false); err != nil {
		t.Fatalf("no-op pair = %v", err)
	}
}

func TestPairRejectsInvalidDependenciesBeforeWrites(t *testing.T) {
	valid := validPair(workingPairDependencies())
	invalidTransaction := valid
	invalidTransaction.Transaction.Parse = nil
	for name, pair := range map[string]Pair{
		"empty directory":        valid,
		"unsafe regular file":    {RegularFile: "../configuration.json", SecretsFile: "secrets.json"},
		"same files":             {RegularFile: "settings.json", SecretsFile: "settings.json"},
		"mismatched transaction": {RegularFile: "configuration.json", SecretsFile: "secrets.json", Transaction: Transaction{RegularFile: "other.json", SecretsFile: "secrets.json"}},
		"invalid transaction":    invalidTransaction,
	} {
		directory := "data"
		if name == "empty directory" {
			directory = ""
		}
		if err := pair.Persist(directory, nil, nil, true, true); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	if err := invalidTransaction.Persist("data", nil, nil, true, false); err != nil {
		t.Fatalf("partial write required an unused transaction: %v", err)
	}
}

func TestPairPropagatesPreparationFailures(t *testing.T) {
	for name, fail := range map[string]func(*pairDependencies){
		"directory": func(dependencies *pairDependencies) {
			dependencies.mkdirAll = func(string, os.FileMode) error { return errPairBlocked }
		},
		"create stage": func(dependencies *pairDependencies) {
			dependencies.createTemp = func(string, string) (StagedFile, error) { return nil, errPairBlocked }
		},
		"write stage": func(dependencies *pairDependencies) {
			dependencies.createTemp = func(string, string) (StagedFile, error) {
				return &stagedFileStub{name: "stage", writeErr: errPairBlocked}, nil
			}
		},
		"read original": func(dependencies *pairDependencies) {
			dependencies.readFile = func(string) ([]byte, error) { return nil, errPairBlocked }
		},
		"write transaction": func(dependencies *pairDependencies) {
			dependencies.writeTransaction = func(string, []byte, []byte) error { return errPairBlocked }
		},
	} {
		t.Run(name, func(t *testing.T) {
			dependencies := workingPairDependencies()
			fail(dependencies)
			if err := validPair(dependencies).Persist("data", nil, nil, true, true); err == nil {
				t.Fatal("failure was not propagated")
			}
		})
	}
}

func TestPairRollsBackReplacementFailures(t *testing.T) {
	for name, configure := range map[string]func(*pairDependencies){
		"rename": func(dependencies *pairDependencies) {
			dependencies.rename = func(string, string) error { return errPairBlocked }
		},
		"restore write": func(dependencies *pairDependencies) {
			reads, renames := 0, 0
			dependencies.readFile = func(string) ([]byte, error) {
				reads++
				if reads == 1 {
					return []byte("old"), nil
				}
				return nil, os.ErrNotExist
			}
			dependencies.rename = func(string, string) error {
				renames++
				if renames == 2 {
					return errPairBlocked
				}
				return nil
			}
			dependencies.writeFile = func(string, []byte, os.FileMode) error { return errPairBlocked }
		},
	} {
		t.Run(name, func(t *testing.T) {
			dependencies := workingPairDependencies()
			configure(dependencies)
			if err := validPair(dependencies).Persist("data", nil, nil, true, true); err == nil {
				t.Fatal("replacement failure was not propagated")
			}
		})
	}
}

func TestPairPropagatesRollbackCleanupFailures(t *testing.T) {
	for name, configure := range map[string]func(*pairDependencies){
		"restore remove": func(dependencies *pairDependencies) {
			renames := 0
			dependencies.rename = func(string, string) error {
				renames++
				if renames == 2 {
					return errPairBlocked
				}
				return nil
			}
			dependencies.remove = func(path string) error {
				if filepath.Base(path) == "configuration.json" {
					return errPairBlocked
				}
				return nil
			}
		},
		"remove marker": func(dependencies *pairDependencies) {
			dependencies.rename = func(string, string) error { return errPairBlocked }
			dependencies.removeTransaction = func(string) error { return errPairBlocked }
		},
	} {
		t.Run(name, func(t *testing.T) {
			dependencies := workingPairDependencies()
			configure(dependencies)
			if err := validPair(dependencies).Persist("data", nil, nil, true, true); err == nil {
				t.Fatal("replacement failure was not propagated")
			}
		})
	}
}

func TestPairCompletesInjectedTransactionAndMissingRollback(t *testing.T) {
	dependencies := workingPairDependencies()
	dependencies.removeTransaction = func(string) error { return errPairBlocked }
	if err := validPair(dependencies).Persist("data", nil, nil, true, true); !errors.Is(err, errPairBlocked) {
		t.Fatalf("remove transaction error = %v", err)
	}
	files := []pairFile{{path: "old", write: true}, {path: "new", write: false}}
	dependencies.remove = func(string) error { return os.ErrNotExist }
	if err := restorePairFiles(files, []bool{true, false}, [][]byte{nil, nil}, []bool{false, false}, dependencies); err != nil {
		t.Fatalf("missing rollback target = %v", err)
	}
}
