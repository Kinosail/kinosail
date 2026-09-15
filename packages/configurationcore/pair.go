package configurationcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Pair durably persists public and secret configuration files.
type Pair struct {
	RegularFile  string
	SecretsFile  string
	Transaction  Transaction
	dependencies *pairDependencies
}

type pairFile struct {
	path, temporary string
	data            []byte
	write           bool
}

type pairDependencies struct {
	mkdirAll          func(string, os.FileMode) error
	createTemp        func(string, string) (StagedFile, error)
	readFile          func(string) ([]byte, error)
	rename            func(string, string) error
	writeFile         func(string, []byte, os.FileMode) error
	remove            func(string) error
	writeTransaction  func(string, []byte, []byte) error
	removeTransaction func(string) error
}

// Persist replaces either or both maps and uses a recovery journal for paired writes.
func (pair Pair) Persist(dataDir string, regular, secrets map[string]string, writeRegular, writeSecrets bool) error {
	if !writeRegular && !writeSecrets {
		return nil
	}
	regularData, _ := json.Marshal(regular)
	secretsData, _ := json.Marshal(secrets)
	return pair.persist(dataDir, regularData, secretsData, writeRegular, writeSecrets)
}

func (pair Pair) persist(dataDir string, regular, secrets []byte, writeRegular, writeSecrets bool) error {
	if err := pair.valid(dataDir, writeRegular && writeSecrets); err != nil {
		return err
	}
	dependencies := pair.deps()
	if err := dependencies.mkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	files := []pairFile{
		{path: filepath.Join(dataDir, pair.RegularFile), data: regular, write: writeRegular},
		{path: filepath.Join(dataDir, pair.SecretsFile), data: secrets, write: writeSecrets},
	}
	if err := stagePairFiles(dataDir, files, dependencies); err != nil {
		return err
	}
	defer removePairStages(files, dependencies)
	originals, existed, err := readPairOriginals(files, dependencies)
	if err != nil {
		return err
	}
	transaction := writeRegular && writeSecrets
	if transaction {
		if err := pair.writeTransaction(dataDir, regular, secrets); err != nil {
			return err
		}
	}
	if err := replacePairFiles(dataDir, files, originals, existed, transaction, pair, dependencies); err != nil {
		return err
	}
	return pair.finishTransaction(dataDir, transaction)
}

func (pair Pair) finishTransaction(dataDir string, transaction bool) error {
	if transaction {
		return pair.removeTransaction(dataDir)
	}
	return nil
}

func (pair Pair) valid(dataDir string, transaction bool) error {
	if dataDir == "" || !safeConfigurationFilename(pair.RegularFile) || !safeConfigurationFilename(pair.SecretsFile) || pair.RegularFile == pair.SecretsFile {
		return errors.New("configuration pair dependencies are invalid")
	}
	if transaction && (pair.Transaction.RegularFile != pair.RegularFile || pair.Transaction.SecretsFile != pair.SecretsFile) {
		return errors.New("configuration pair transaction does not match")
	}
	if transaction {
		return pair.Transaction.valid(dataDir)
	}
	return nil
}

func (pair Pair) deps() *pairDependencies {
	if pair.dependencies != nil {
		return pair.dependencies
	}
	return &pairDependencies{
		mkdirAll: os.MkdirAll,
		createTemp: func(directory, pattern string) (StagedFile, error) {
			return os.CreateTemp(directory, pattern)
		},
		readFile:  os.ReadFile,
		rename:    os.Rename,
		writeFile: os.WriteFile,
		remove:    os.Remove,
	}
}

func (pair Pair) writeTransaction(dataDir string, regular, secrets []byte) error {
	if pair.dependencies != nil && pair.dependencies.writeTransaction != nil {
		return pair.dependencies.writeTransaction(dataDir, regular, secrets)
	}
	return pair.Transaction.Write(dataDir, regular, secrets)
}

func (pair Pair) removeTransaction(dataDir string) error {
	if pair.dependencies != nil && pair.dependencies.removeTransaction != nil {
		return pair.dependencies.removeTransaction(dataDir)
	}
	return pair.Transaction.Remove(dataDir)
}

func stagePairFiles(dataDir string, files []pairFile, dependencies *pairDependencies) error {
	for index := range files {
		if !files[index].write {
			continue
		}
		file, err := dependencies.createTemp(dataDir, ".kinosail-configuration-*")
		if err != nil {
			removePairStages(files, dependencies)
			return err
		}
		files[index].temporary = file.Name()
		if err := WriteStagedFile(file, files[index].data, 0o600); err != nil {
			removePairStages(files, dependencies)
			return err
		}
	}
	return nil
}

func removePairStages(files []pairFile, dependencies *pairDependencies) {
	for _, file := range files {
		if file.temporary != "" {
			_ = dependencies.remove(file.temporary)
		}
	}
}

func readPairOriginals(files []pairFile, dependencies *pairDependencies) ([][]byte, []bool, error) {
	originals := make([][]byte, len(files))
	existed := make([]bool, len(files))
	for index := range files {
		if !files[index].write {
			continue
		}
		data, err := dependencies.readFile(files[index].path)
		if err == nil {
			originals[index], existed[index] = data, true
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
	}
	return originals, existed, nil
}

func replacePairFiles(dataDir string, files []pairFile, originals [][]byte, existed []bool, transaction bool, pair Pair, dependencies *pairDependencies) error {
	renamed := make([]bool, len(files))
	for index := range files {
		if !files[index].write {
			continue
		}
		if err := dependencies.rename(files[index].temporary, files[index].path); err != nil {
			if restoreErr := restorePairFiles(files, renamed, originals, existed, dependencies); restoreErr != nil {
				return fmt.Errorf("persist configuration: %w; rollback: %w", err, restoreErr)
			}
			if transaction {
				if transactionErr := pair.removeTransaction(dataDir); transactionErr != nil {
					return fmt.Errorf("persist configuration: %w; rollback marker: %w", err, transactionErr)
				}
			}
			return err
		}
		renamed[index] = true
	}
	return nil
}

func restorePairFiles(files []pairFile, renamed []bool, originals [][]byte, existed []bool, dependencies *pairDependencies) error {
	for index := range files {
		if !renamed[index] {
			continue
		}
		if existed[index] {
			if err := dependencies.writeFile(files[index].path, originals[index], 0o600); err != nil {
				return err
			}
		} else if err := dependencies.remove(files[index].path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
