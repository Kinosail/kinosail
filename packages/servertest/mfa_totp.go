package servertest

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // TOTP is specified with HMAC-SHA-1 by RFC 6238.
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"
	"time"
)

var (
	issuedTOTPMu sync.Mutex
	issuedTOTP   = map[string]int64{}
)

func TestTOTP(t *testing.T, secret string, now time.Time) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return totpForCounter(key, now.Unix()/30)
}

func NextTOTP(t *testing.T, secret string) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return totpForCounter(key, nextTOTPCounter(secret))
}

func nextTOTPCounter(secret string) int64 {
	for {
		now := time.Now()
		if now.Unix()%30 >= 27 {
			time.Sleep(time.Until(time.Unix((now.Unix()/30+1)*30, 0)))
			continue
		}
		window := now.Unix() / 30
		issuedTOTPMu.Lock()
		counter := max(window-1, issuedTOTP[secret]+1)
		if counter <= window+1 {
			issuedTOTP[secret] = counter
			issuedTOTPMu.Unlock()
			return counter
		}
		issuedTOTPMu.Unlock()
		time.Sleep(time.Until(time.Unix((window+1)*30, 0)))
	}
}

func totpForCounter(key []byte, counterValue int64) string {
	var counter [8]byte
	//nolint:gosec // Test vectors use known post-epoch timestamps.
	binary.BigEndian.PutUint64(counter[:], uint64(counterValue))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	value := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1_000_000)
}
