package backup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/servertest"
	"golang.org/x/crypto/scrypt"
)

func TestBackupRejectsSemanticInvalidSettingsBeforeArchiveOutput(t *testing.T) {
	twentyOne := `["en","fr","de","es","it","nl","pl","pt","ru","uk","tr","ar","fa","he","hi","bn","ur","id","ms","vi","th"]`
	for name, settings := range map[string]string{
		"empty":     `{"subtitleLanguages":[]}`,
		"too many":  `{"subtitleLanguages":` + twentyOne + `}`,
		"duplicate": `{"subtitleLanguages":["pt-br","pt-BR"]}`,
		"overlap":   `{"subtitleLanguages":["pt","pt-BR"]}`,
		"unknown":   `{"subtitleLanguages":["not-a-language"]}`,
		"provider":  `{"subtitleLanguages":["ea"]}`,
		"conflict":  `{"subtitleLanguage":"fr","subtitleLanguages":["en","fr"]}`,
		"supporter": `{"supporter":{"patronOrder":{"unexpected":true}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			writeSemanticBackupFile(t, filepath.Join(directory, "api_keys.json"), `{}`)
			writeSemanticBackupFile(t, filepath.Join(directory, "settings.json"), settings)
			for adapter, write := range map[string]func(io.Writer) error{
				"plain": func(output io.Writer) error { return service.WriteAuto(output, directory, "", ApplicationVersion) },
				"encrypted": func(output io.Writer) error {
					return service.WriteAuto(output, directory, "correct horse battery staple", ApplicationVersion)
				},
			} {
				t.Run(adapter, func(t *testing.T) {
					var output bytes.Buffer
					if err := write(&output); err == nil || output.Len() != 0 {
						t.Fatalf("invalid settings archive = %d bytes, %v", output.Len(), err)
					}
				})
			}
		})
	}
}

func TestRestoreAndVerifyRejectSemanticInvalidSettingsBeforeReplacement(t *testing.T) { //nolint:gocognit // Both archive modes prove rejection is side-effect free.
	invalid := servertest.TarGzipFile(t, "settings.json", `{"supporter":{"livingStandard":[]}}`)
	encrypted := encryptTestArchive(t, invalid, "correct horse battery staple")
	for name, test := range map[string]struct {
		archive    []byte
		passphrase string
	}{
		"plain":     {invalid, ""},
		"encrypted": {encrypted, "correct horse battery staple"},
	} {
		t.Run(name, func(t *testing.T) {
			destination := t.TempDir()
			original := []byte(`{"subtitleLanguages":["en"]}`)
			writeSemanticBackupFile(t, filepath.Join(destination, "settings.json"), string(original))
			if err := service.VerifyAuto(bytes.NewReader(test.archive), test.passphrase); err == nil {
				t.Fatal("invalid settings archive verified")
			}
			if err := service.RestoreAuto(bytes.NewReader(test.archive), destination, test.passphrase); err == nil {
				t.Fatal("invalid settings archive restored")
			}
			current, err := os.ReadFile(filepath.Join(destination, "settings.json"))
			if err != nil || !bytes.Equal(current, original) {
				t.Fatalf("rejected restore changed destination: %q, %v", current, err)
			}
			for _, suffix := range []string{"", "-wal", "-shm"} {
				if _, err = os.Lstat(filepath.Join(destination, database.Filename+suffix)); !os.IsNotExist(err) {
					t.Fatalf("rejected restore created database file %q: %v", suffix, err)
				}
			}
		})
	}
}

func writeSemanticBackupFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func encryptTestArchive(t *testing.T, plain []byte, passphrase string) []byte {
	t.Helper()
	magic := []byte("KINOSAIL-BACKUP-1\n")
	salt, nonce := bytes.Repeat([]byte{1}, 16), bytes.Repeat([]byte{2}, 12)
	key, err := scrypt.Key([]byte(passphrase), salt, 32768, 8, 1, 32)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	result := append(append(append([]byte{}, magic...), salt...), nonce...)
	return append(result, aead.Seal(nil, nonce, plain, magic)...)
}
