package backup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"io"

	"golang.org/x/crypto/scrypt"
)

var encryptedMagic = []byte("KINOSAIL-BACKUP-1\n")

const (
	maxArchiveSize    = 40 << 20
	maxPassphraseSize = 4096
)

// WriteAuto includes private state when an installation backup key is available.
func (service *Service) WriteAuto(writer io.Writer, dataDir, passphrase, version string) error {
	if passphrase != "" {
		return service.WriteEncrypted(writer, dataDir, passphrase, version)
	}
	return service.Write(writer, dataDir, version)
}

// WriteEncrypted creates an authenticated encrypted configuration archive.
func (service *Service) WriteEncrypted(writer io.Writer, dataDir, passphrase, version string) error { //nolint:cyclop // Encryption must fail closed at every archive and cipher boundary.
	if len(passphrase) < 16 || len(passphrase) > maxPassphraseSize {
		return errors.New("backup passphrase must contain 16 to 4096 characters")
	}
	var plain bytes.Buffer
	if err := service.write(&plain, dataDir, version, true); err != nil {
		return err
	}
	salt, nonce := make([]byte, 16), make([]byte, 12)
	if _, err := io.ReadFull(service.random, salt); err != nil {
		return err
	}
	if _, err := io.ReadFull(service.random, nonce); err != nil {
		return err
	}
	key, _ := scrypt.Key([]byte(passphrase), salt, 32768, 8, 1, 32)
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	var err error
	if _, err = writer.Write(encryptedMagic); err == nil {
		_, err = writer.Write(salt)
	}
	if err == nil {
		_, err = writer.Write(nonce)
	}
	if err == nil {
		_, err = writer.Write(aead.Seal(nil, nonce, plain.Bytes(), encryptedMagic))
	}
	return err
}

// RestoreEncrypted authenticates and restores an encrypted archive.
func (service *Service) RestoreEncrypted(reader io.Reader, dataDir, passphrase string) error {
	return service.decrypt(reader, passphrase, func(plain []byte) error {
		staged, err := service.read(bytes.NewReader(plain), true)
		if err != nil {
			return err
		}
		return service.save(staged, dataDir, append(files, secretFiles...))
	})
}

func (service *Service) decrypt(reader io.Reader, passphrase string, use func([]byte) error) error { //nolint:cyclop // Each authenticated-encryption boundary must fail closed.
	if len(passphrase) < 16 || len(passphrase) > maxPassphraseSize {
		return errors.New("backup passphrase must contain 16 to 4096 characters")
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxArchiveSize+1))
	header := len(encryptedMagic) + 16 + 12
	if err != nil || len(data) > maxArchiveSize || len(data) < header || !bytes.Equal(data[:len(encryptedMagic)], encryptedMagic) {
		return errors.New("encrypted backup is invalid")
	}
	salt, nonce := data[len(encryptedMagic):len(encryptedMagic)+16], data[len(encryptedMagic)+16:header]
	key, _ := scrypt.Key([]byte(passphrase), salt, 32768, 8, 1, 32)
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	plain, err := aead.Open(nil, nonce, data[header:], encryptedMagic)
	if err != nil {
		return errors.New("encrypted backup authentication failed")
	}
	return use(plain)
}

// VerifyEncrypted authenticates and validates an encrypted backup without restoring it.
func (service *Service) VerifyEncrypted(reader io.Reader, passphrase string) error {
	return service.decrypt(reader, passphrase, func(plain []byte) error {
		_, err := service.read(bytes.NewReader(plain), true)
		return err
	})
}

// VerifyAuto validates either a plain archive or an authenticated encrypted archive.
func (service *Service) VerifyAuto(reader io.Reader, passphrase string) error {
	data, err := io.ReadAll(io.LimitReader(reader, maxArchiveSize+1))
	if err != nil || len(data) > maxArchiveSize {
		if err == nil {
			err = errors.New("backup exceeds maximum size")
		}
		return err
	}
	if bytes.HasPrefix(data, encryptedMagic) {
		return service.VerifyEncrypted(bytes.NewReader(data), passphrase)
	}
	_, err = service.read(bytes.NewReader(data), false)
	return err
}

// RestoreAuto restores either a plain archive or an authenticated encrypted archive.
func (service *Service) RestoreAuto(reader io.Reader, dataDir, passphrase string) error {
	data, err := io.ReadAll(io.LimitReader(reader, maxArchiveSize+1))
	if err != nil || len(data) > maxArchiveSize {
		if err == nil {
			err = errors.New("backup exceeds maximum size")
		}
		return err
	}
	if bytes.HasPrefix(data, encryptedMagic) {
		return service.RestoreEncrypted(bytes.NewReader(data), dataDir, passphrase)
	}
	return service.Restore(bytes.NewReader(data), dataDir)
}
