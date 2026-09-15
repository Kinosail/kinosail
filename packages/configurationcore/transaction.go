package configurationcore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

const (
	// TransactionFilename is the durable two-file recovery journal.
	TransactionFilename = ".kinosail-configuration-transaction"
	// MaxTransactionSize bounds a configuration recovery journal.
	MaxTransactionSize = 3 << 20
)

type transactionEnvelope struct {
	Regular []byte `json:"regular"`
	Secrets []byte `json:"secrets"`
}

// Transaction owns atomic recovery for Player's public and secret configuration pair.
type Transaction struct {
	RegularFile string
	SecretsFile string
	Parse       func([]byte, string) (map[string]string, error)
	Validate    func(map[string]string, map[string]string) error
}

// Write writes one complete recovery journal before the paired files are replaced.
func (transaction Transaction) Write(dataDir string, regular, secrets []byte) error {
	if err := transaction.valid(dataDir); err != nil {
		return err
	}
	if len(regular) == 0 || len(secrets) == 0 {
		return errors.New("configuration transaction is incomplete")
	}
	data, _ := json.Marshal(transactionEnvelope{Regular: regular, Secrets: secrets})
	if len(data) > MaxTransactionSize {
		return errors.New("configuration transaction is too large")
	}
	return privatefile.Write(filepath.Join(dataDir, TransactionFilename), data)
}

// Recover validates and restores a pending configuration transaction.
func (transaction Transaction) Recover(dataDir string) error {
	if err := transaction.valid(dataDir); err != nil {
		return err
	}
	data, err := privatefile.Read(filepath.Join(dataDir, TransactionFilename), MaxTransactionSize)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	regular, secrets, err := transaction.read(data)
	if err != nil {
		return fmt.Errorf("read configuration transaction: %w", err)
	}
	if err := privatefile.Write(filepath.Join(dataDir, transaction.RegularFile), regular); err != nil {
		return err
	}
	if err := privatefile.Write(filepath.Join(dataDir, transaction.SecretsFile), secrets); err != nil {
		return err
	}
	return privatefile.Remove(filepath.Join(dataDir, TransactionFilename))
}

// Remove removes a recovery journal after the paired replacement commits or rolls back.
func (transaction Transaction) Remove(dataDir string) error {
	if err := transaction.valid(dataDir); err != nil {
		return err
	}
	return privatefile.Remove(filepath.Join(dataDir, TransactionFilename))
}

func (transaction Transaction) valid(dataDir string) error {
	if dataDir == "" || !safeConfigurationFilename(transaction.RegularFile) || !safeConfigurationFilename(transaction.SecretsFile) || transaction.RegularFile == transaction.SecretsFile || transaction.Parse == nil || transaction.Validate == nil {
		return errors.New("configuration transaction dependencies are invalid")
	}
	return nil
}

func safeConfigurationFilename(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name
}

func (transaction Transaction) read(data []byte) ([]byte, []byte, error) {
	envelope, err := decodeTransaction(data)
	if err != nil {
		return nil, nil, err
	}
	regular, err := transaction.Parse(envelope.Regular, transaction.RegularFile)
	if err != nil {
		return nil, nil, err
	}
	secrets, err := transaction.Parse(envelope.Secrets, transaction.SecretsFile)
	if err != nil {
		return nil, nil, err
	}
	if err := transaction.Validate(regular, secrets); err != nil {
		return nil, nil, err
	}
	regularData, _ := json.Marshal(regular)
	secretsData, _ := json.Marshal(secrets)
	return regularData, secretsData, nil
}

func decodeTransaction(data []byte) (transactionEnvelope, error) { //nolint:cyclop,gocognit // The strict envelope parser rejects every ambiguous journal shape before recovery writes.
	if len(data) > MaxTransactionSize || !utf8.Valid(data) {
		return transactionEnvelope{}, errors.New("transaction is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return transactionEnvelope{}, errors.New("transaction must be one JSON object")
	}
	envelope := transactionEnvelope{}
	seen := make(map[string]bool, 2)
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return transactionEnvelope{}, errors.New("transaction fields are invalid")
		}
		seen[key] = true
		switch key {
		case "regular":
			err = decoder.Decode(&envelope.Regular)
		case "secrets":
			err = decoder.Decode(&envelope.Secrets)
		default:
			return transactionEnvelope{}, errors.New("transaction field is unknown")
		}
		if err != nil {
			return transactionEnvelope{}, errors.New("transaction field is invalid")
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return transactionEnvelope{}, errors.New("transaction object is incomplete")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return transactionEnvelope{}, errors.New("transaction has trailing data")
	}
	if len(envelope.Regular) == 0 || len(envelope.Secrets) == 0 {
		return transactionEnvelope{}, errors.New("transaction is incomplete")
	}
	return envelope, nil
}
