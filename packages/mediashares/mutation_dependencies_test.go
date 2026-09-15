package mediashares

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestDependencyValidationRequiresEveryAdapter(t *testing.T) {
	valid := Dependencies{
		Load: func(string, any) (bool, error) { return false, nil }, Persist: func(string, any) error { return nil },
		Find: func(string) (library.Item, bool) { return library.Item{}, false }, Snapshot: func() ([]library.Item, error) { return nil, nil },
	}
	for name, mutate := range map[string]func(*Dependencies){
		"load": func(value *Dependencies) { value.Load = nil }, "persist": func(value *Dependencies) { value.Persist = nil },
		"find": func(value *Dependencies) { value.Find = nil }, "snapshot": func(value *Dependencies) { value.Snapshot = nil },
	} {
		value := valid
		mutate(&value)
		if _, err := normalizedDependencies(value); err == nil {
			t.Errorf("missing %s was accepted", name)
		}
	}
	value, err := normalizedDependencies(valid)
	if err != nil || value.Now == nil || value.Random == nil {
		t.Fatalf("valid dependencies = %#v, %v", value, err)
	}
}
