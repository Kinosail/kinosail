package configurationcore

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

var errRejectedTransaction = errors.New("rejected transaction")

func transactionFixture() Transaction {
	return Transaction{
		RegularFile: "configuration.json",
		SecretsFile: "secrets.json",
		Parse: func(data []byte, _ string) (map[string]string, error) {
			if string(data) == "bad" {
				return nil, errRejectedTransaction
			}
			var values map[string]string
			if err := json.Unmarshal(data, &values); err != nil || values == nil {
				return nil, errRejectedTransaction
			}
			return values, nil
		},
		Validate: func(regular, _ map[string]string) error {
			if regular["reject"] != "" {
				return errRejectedTransaction
			}
			return nil
		},
	}
}

func TestTransactionWritesRecoversAndRemovesThePair(t *testing.T) {
	t.Parallel()
	transaction := transactionFixture()
	directory := t.TempDir()
	if err := transaction.Recover(directory); err != nil {
		t.Fatalf("Recover(missing) error = %v", err)
	}
	regular := []byte(`{"name":"Player"}`)
	secrets := []byte(`{"token":"private"}`)
	if err := transaction.Write(directory, regular, secrets); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Recover(directory); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{transaction.RegularFile: regular, transaction.SecretsFile: secrets} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || !bytes.Equal(data, want) {
			t.Fatalf("recovered %s = %q, error %v", name, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, TransactionFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("transaction remains: %v", err)
	}
	if err := transaction.Remove(directory); err != nil {
		t.Fatalf("Remove(missing) error = %v", err)
	}
}

func TestTransactionRejectsInvalidWritesBeforeCreatingAJournal(t *testing.T) {
	t.Parallel()
	valid := transactionFixture()
	invalid := map[string]Transaction{
		"regular empty":      {SecretsFile: valid.SecretsFile, Parse: valid.Parse, Validate: valid.Validate},
		"regular dot":        {RegularFile: ".", SecretsFile: valid.SecretsFile, Parse: valid.Parse, Validate: valid.Validate},
		"regular parent":     {RegularFile: "..", SecretsFile: valid.SecretsFile, Parse: valid.Parse, Validate: valid.Validate},
		"regular path":       {RegularFile: "nested/configuration.json", SecretsFile: valid.SecretsFile, Parse: valid.Parse, Validate: valid.Validate},
		"secrets empty":      {RegularFile: valid.RegularFile, Parse: valid.Parse, Validate: valid.Validate},
		"matching names":     {RegularFile: valid.RegularFile, SecretsFile: valid.RegularFile, Parse: valid.Parse, Validate: valid.Validate},
		"missing parser":     {RegularFile: valid.RegularFile, SecretsFile: valid.SecretsFile, Validate: valid.Validate},
		"missing validation": {RegularFile: valid.RegularFile, SecretsFile: valid.SecretsFile, Parse: valid.Parse},
	}
	for name, transaction := range invalid {
		if err := transaction.Write(t.TempDir(), []byte(`{}`), []byte(`{}`)); err == nil {
			t.Fatalf("%s transaction was accepted", name)
		}
	}
	directory := t.TempDir()
	for name, pair := range map[string][2][]byte{
		"missing regular": {nil, []byte(`{}`)},
		"missing secrets": {[]byte(`{}`), nil},
		"oversized":       {bytes.Repeat([]byte("x"), MaxTransactionSize), []byte(`{}`)},
	} {
		if err := valid.Write(directory, pair[0], pair[1]); err == nil {
			t.Fatalf("%s transaction was accepted", name)
		}
		if _, err := os.Stat(filepath.Join(directory, TransactionFilename)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s created a journal: %v", name, err)
		}
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := valid.Write(filepath.Join(file, "child"), []byte(`{}`), []byte(`{}`)); err == nil {
		t.Fatal("journal under a regular file was accepted")
	}
}

func TestTransactionRejectsEveryInvalidEnvelopeWithoutSideEffects(t *testing.T) {
	t.Parallel()
	transaction := transactionFixture()
	for name, data := range map[string][]byte{
		"invalid UTF-8":     {0xff},
		"not object":        []byte(`[]`),
		"malformed":         []byte(`{"regular"`),
		"duplicate":         []byte(`{"regular":"e30=","regular":"e30=","secrets":"e30="}`),
		"unknown":           []byte(`{"regular":"e30=","secrets":"e30=","extra":"e30="}`),
		"invalid field":     []byte(`{"regular":{},"secrets":"e30="}`),
		"incomplete":        []byte(`{"regular":"e30="`),
		"object incomplete": []byte(`{"regular":"e30=","secrets":"e30="`),
		"wrong close":       []byte(`{"regular":"e30=","secrets":"e30="]`),
		"trailing":          []byte(`{"regular":"e30=","secrets":"e30="} {}`),
		"missing secrets":   []byte(`{"regular":"e30="}`),
		"bad regular":       envelope(t, []byte("bad"), []byte(`{}`)),
		"bad secrets":       envelope(t, []byte(`{}`), []byte("bad")),
		"bad semantics":     envelope(t, []byte(`{"reject":"yes"}`), []byte(`{}`)),
	} {
		t.Run(name, func(t *testing.T) {
			assertRejectedTransactionPreservesFiles(t, transaction, data)
		})
	}
	if _, err := decodeTransaction(bytes.Repeat([]byte("x"), MaxTransactionSize+1)); err == nil {
		t.Fatal("oversized envelope was accepted")
	}
}

func TestTransactionRecoverySurvivesTargetAndJournalFailures(t *testing.T) {
	t.Parallel()
	transaction := transactionFixture()
	for _, target := range []string{transaction.RegularFile, transaction.SecretsFile} {
		assertTransactionRecoveryAfterTargetRepair(t, transaction, target)
	}
	directory := t.TempDir()
	marker := filepath.Join(directory, TransactionFilename)
	if err := os.Mkdir(marker, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(marker, "entry"), []byte("entry"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Recover(directory); err == nil {
		t.Fatal("journal directory was accepted")
	}
	if err := transaction.Remove(directory); err == nil {
		t.Fatal("journal directory was removed")
	}
	invalid := Transaction{}
	if err := invalid.Recover(directory); err == nil {
		t.Fatal("invalid recovery dependencies were accepted")
	}
	if err := invalid.Remove(directory); err == nil {
		t.Fatal("invalid removal dependencies were accepted")
	}
}

func envelope(t *testing.T, regular, secrets []byte) []byte {
	t.Helper()
	data, err := json.Marshal(transactionEnvelope{Regular: regular, Secrets: secrets})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertRejectedTransactionPreservesFiles(t *testing.T, transaction Transaction, data []byte) {
	t.Helper()
	directory := t.TempDir()
	regularPath := filepath.Join(directory, transaction.RegularFile)
	secretsPath := filepath.Join(directory, transaction.SecretsFile)
	regularBefore, secretsBefore := []byte(`{"before":"regular"}`), []byte(`{"before":"secret"}`)
	if err := os.WriteFile(regularPath, regularBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretsPath, secretsBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(directory, TransactionFilename)
	if err := os.WriteFile(marker, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Recover(directory); err == nil {
		t.Fatal("invalid transaction was accepted")
	}
	for path, want := range map[string][]byte{regularPath: regularBefore, secretsPath: secretsBefore, marker: data} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("recovery changed %s to %q, error %v", path, got, err)
		}
	}
}

func assertTransactionRecoveryAfterTargetRepair(t *testing.T, transaction Transaction, target string) {
	t.Helper()
	directory := t.TempDir()
	if err := transaction.Write(directory, []byte(`{}`), []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Recover(directory); err == nil {
		t.Fatalf("recovery with %s target directory succeeded", target)
	}
	if err := os.Remove(filepath.Join(directory, target)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Recover(directory); err != nil {
		t.Fatalf("recovery after %s repair failed: %v", target, err)
	}
}
