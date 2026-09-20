package auditjournal

import (
	"strconv"
	"testing"
)

func TestQueryClampsExtremeLimitsAndReturnsNewestEvents(t *testing.T) {
	journal := &Journal{}
	for index := range 1001 {
		journal.events = append(journal.events, Event{ID: strconv.Itoa(index)})
	}
	for _, tc := range []struct{ limit, want int }{{-int(^uint(0)>>1) - 1, 1}, {0, 1}, {1, 1}, {1000, 1000}, {int(^uint(0) >> 1), 1000}} {
		got := journal.Query("", tc.limit)
		if len(got) != tc.want || got[0].ID != "1000" {
			t.Fatalf("limit %d returned %d events", tc.limit, len(got))
		}
	}
}
