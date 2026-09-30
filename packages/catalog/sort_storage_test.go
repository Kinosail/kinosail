package catalog_test

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestLongLocaleTitlesStayOrderedAcrossPages(t *testing.T) {
	families := []string{"Alpha", "Zebra", "Åland", "Ängel", "Öland"}
	items := make([]library.Item, 0, 640)
	want := make([]string, 0, 640)
	for _, family := range families {
		for index := range 128 {
			id := fmt.Sprintf("%s-%03d", family, index)
			want = append(want, id)
			items = append(items, library.Item{ID: id, Title: id,
				SortTitle: family + strings.Repeat("é", 900) + fmt.Sprintf(" %03d", index/2)})
		}
	}
	candidates := make([]catalog.Candidate, len(items))
	for index := range items {
		candidates[len(items)-1-index] = catalog.Candidate{Item: &items[index]}
	}
	var got []string
	for offset := 0; offset < len(items); offset += 200 {
		browse, err := catalog.ParseBrowse(url.Values{"offset": {strconv.Itoa(offset)}, "limit": {"200"}}, "sv")
		if err != nil {
			t.Fatal(err)
		}
		result, err := browse.Apply(candidates)
		if err != nil || result.Total != len(items) {
			t.Fatalf("page %d: total %d, error %v", offset, result.Total, err)
		}
		for _, item := range result.Items {
			got = append(got, item.ID)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("locale ordering, equal-title ID ties, or page boundaries changed")
	}
	if candidates[0].Item.ID != want[len(want)-1] {
		t.Fatal("browse mutated its input order")
	}
}
