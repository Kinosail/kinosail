package servertest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

// WebAuthnDevice creates real signed credentials for application handler tests.
type WebAuthnDevice struct {
	key          *ecdsa.PrivateKey
	id           []byte
	idLength     uint16
	rpID, origin string
}

// NewWebAuthnDevice binds a local test authenticator to an explicit relying party.
func NewWebAuthnDevice(t testing.TB, rpID, origin, credentialID string) WebAuthnDevice {
	t.Helper()
	size := len(credentialID)
	if size < 1 || size > 65535 {
		t.Fatal("invalid test credential ID length")
		return WebAuthnDevice{}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return WebAuthnDevice{key: key, id: []byte(credentialID), idLength: uint16(size), rpID: rpID, origin: origin}
}

// Registration supplies a discoverable ES256 credential with user verification.
func (device WebAuthnDevice) Registration(t testing.TB, challenge string) string {
	t.Helper()
	publicKey := map[int]any{1: 2, 3: -7, -1: 1, -2: device.key.X.FillBytes(make([]byte, 32)), -3: device.key.Y.FillBytes(make([]byte, 32))}
	data := device.authenticatorData(0x45, 0)
	data = append(data, make([]byte, 16)...)
	data = binary.BigEndian.AppendUint16(data, device.idLength)
	data = append(data, device.id...)
	data = append(data, webAuthnCBOR(t, publicKey)...)
	attestation := webAuthnCBOR(t, map[string]any{"fmt": "none", "authData": data, "attStmt": map[string]any{}})
	return device.credentialJSON(t, map[string]any{
		"attestationObject": base64.RawURLEncoding.EncodeToString(attestation),
		"clientDataJSON":    base64.RawURLEncoding.EncodeToString(device.clientData(t, "webauthn.create", challenge)),
	})
}

// Assertion signs the challenge and the relying party data with the device key.
func (device WebAuthnDevice) Assertion(t testing.TB, challenge, owner string) string {
	t.Helper()
	clientData := device.clientData(t, "webauthn.get", challenge)
	authData := device.authenticatorData(0x05, 1)
	clientHash := sha256.Sum256(clientData)
	message := append(append([]byte(nil), authData...), clientHash[:]...)
	digest := sha256.Sum256(message)
	signature, err := ecdsa.SignASN1(rand.Reader, device.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return device.credentialJSON(t, map[string]any{
		"clientDataJSON":    base64.RawURLEncoding.EncodeToString(clientData),
		"authenticatorData": base64.RawURLEncoding.EncodeToString(authData),
		"signature":         base64.RawURLEncoding.EncodeToString(signature),
		"userHandle":        base64.RawURLEncoding.EncodeToString([]byte(owner)),
	})
}

func (device WebAuthnDevice) authenticatorData(flags byte, counter uint32) []byte {
	rpHash := sha256.Sum256([]byte(device.rpID))
	data := append([]byte(nil), rpHash[:]...)
	data = append(data, flags)
	return binary.BigEndian.AppendUint32(data, counter)
}

func (device WebAuthnDevice) clientData(t testing.TB, kind, challenge string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]string{"type": kind, "challenge": challenge, "origin": device.origin})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (device WebAuthnDevice) credentialJSON(t testing.TB, response map[string]any) string {
	t.Helper()
	encoded := base64.RawURLEncoding.EncodeToString(device.id)
	data, err := json.Marshal(map[string]any{"id": encoded, "rawId": encoded, "type": "public-key", "response": response})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func webAuthnCBOR(t testing.TB, value any) []byte {
	t.Helper()
	data, err := cbor.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
