package servertest

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // TOTP is specified with HMAC-SHA-1 by RFC 6238.
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"testing"
	"time"
)

// TestTOTP produces the canonical RFC 6238 code at the supplied test time.
func TestTOTP(t *testing.T, secret string, now time.Time) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	var counter [8]byte
	//nolint:gosec // Test vectors use known post-epoch timestamps.
	binary.BigEndian.PutUint64(counter[:], uint64(now.Unix()/30))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	value := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1_000_000)
}
