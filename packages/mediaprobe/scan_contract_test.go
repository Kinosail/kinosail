package mediaprobe

import (
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func assertStaleScanRejected(t *testing.T, item library.Item, read func(library.Item) (Result, bool)) {
	t.Helper()
	for _, change := range []func(*library.Item){
		func(item *library.Item) { item.Size++ },
		func(item *library.Item) { item.Added = item.Added.Add(time.Second) },
		func(item *library.Item) { item.Size = -1 },
		func(item *library.Item) { item.Added = time.Time{} },
	} {
		changed := item
		change(&changed)
		if _, found := read(changed); found {
			t.Fatal("accepted invalid or changed scan stamp")
		}
	}
}
