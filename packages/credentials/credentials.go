// Package credentials owns Kinosail's local password policy and credential format.
package credentials

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	memory                 = 19 * 1024
	iterations             = 2
	parallelism            = 1
	keyLength              = 32
	maximumPasswordBytes   = 1024
	maximumCredentialBytes = 4096
	parameterString        = "m=19456,t=2,p=1"
)

//go:embed common_passwords.txt.gz.b64
var commonPasswords string

var unsafePasswords = loadCommonPasswords(commonPasswords)

func loadCommonPasswords(source string) map[string]struct{} {
	compressed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(source))
	if err != nil {
		panic(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		panic(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		panic("invalid common password list")
	}
	_ = reader.Close()
	passwords := make(map[string]struct{}, 3006)
	for _, password := range strings.Split(string(decoded), "\n") {
		password = strings.TrimSpace(password)
		if password != "" {
			passwords[password] = struct{}{}
		}
	}
	for _, password := range []string{"123456789012", "adminadminadmin", "changemechangeme", "letmeinletmein", "passwordpassword", "qwertyqwertyqwerty"} {
		passwords[password] = struct{}{}
	}
	if len(passwords) < 3000 {
		panic("common password list is incomplete")
	}
	return passwords
}

var dummyHash = sync.OnceValue(func() string {
	value, _ := hash("constant-time-dummy-password")
	return value
})

// Validate applies Kinosail's password policy without changing state.
func Validate(password string) error {
	if len(password) < 12 {
		return errors.New("password must contain at least 12 characters")
	}
	if len(password) > maximumPasswordBytes {
		return errors.New("password must contain no more than 1024 bytes")
	}
	if _, unsafe := unsafePasswords[strings.ToLower(strings.TrimSpace(password))]; unsafe {
		return errors.New("password is too common")
	}
	return nil
}

// Hash validates and encodes a password with Kinosail's current Argon2id policy.
func Hash(password string) (string, error) {
	if err := Validate(password); err != nil {
		return "", err
	}
	return hash(password)
}

func hash(password string) (string, error) {
	return hashWithReader(password, rand.Reader)
}

func hashWithReader(password string, random io.Reader) (string, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(random, salt); err != nil {
		return "", err
	}
	digest := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return fmt.Sprintf("$argon2id$v=%d$%s$%s$%s", argon2.Version, parameterString, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(digest)), nil
}

// Verify accepts the current PHC format and Kinosail's legacy salt.hash format.
func Verify(encoded, password string) bool {
	if len(encoded) > maximumCredentialBytes || len(password) > maximumPasswordBytes {
		return false
	}
	if strings.HasPrefix(encoded, "$argon2id$") {
		return verifyArgon2id(encoded, password)
	}
	parts := strings.Split(encoded, ".")
	if len(parts) != 2 {
		return false
	}
	salt, saltErr := base64.RawStdEncoding.DecodeString(parts[0])
	want, hashErr := base64.RawStdEncoding.DecodeString(parts[1])
	if saltErr != nil || hashErr != nil || len(want) != keyLength {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// DummyVerify performs equivalent password work when no profile can be checked.
func DummyVerify(password string) {
	_ = Verify(dummyHash(), password)
}

func verifyArgon2id(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) || parts[3] != parameterString {
		return false
	}
	encoding := base64.RawStdEncoding.Strict()
	salt, saltErr := encoding.DecodeString(parts[4])
	want, hashErr := encoding.DecodeString(parts[5])
	if saltErr != nil || hashErr != nil || len(salt) != 16 || len(want) != keyLength {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return subtle.ConstantTimeCompare(got, want) == 1
}
