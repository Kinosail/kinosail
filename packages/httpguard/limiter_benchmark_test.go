package httpguard

import (
	"strconv"
	"testing"
)

func BenchmarkLimiterAllowNewClientWithFullTable(b *testing.B) {
	var limiter Limiter
	for client := range trackedClientLimit {
		limiter.Allow("client-"+strconv.Itoa(client), 10)
	}
	keys := make([]string, 1<<16)
	for index := range keys {
		keys[index] = "203.0." + strconv.Itoa(index>>8) + "." + strconv.Itoa(index&0xff)
	}
	index := 0
	b.ReportAllocs()
	for b.Loop() {
		limiter.Allow(keys[index&(len(keys)-1)], 10)
		index++
	}
}
