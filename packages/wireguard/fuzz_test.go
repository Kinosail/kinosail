package wireguard

import "testing"

func FuzzPersistedStateValidation(f *testing.F) {
	f.Add("private", "public", "viewer", "profile", "peer", "psk", "10.91.0.2")
	f.Add("", "", "", "", "", "", "")
	f.Fuzz(func(t *testing.T, privateKey, publicKey, label, profileID, peerKey, presharedKey, address string) {
		_ = validState(state{PrivateKey: privateKey, PublicKey: publicKey, Peers: []Viewer{{Label: label, ProfileID: profileID, PublicKey: peerKey, PresharedKey: presharedKey, Address: address}}})
	})
}
