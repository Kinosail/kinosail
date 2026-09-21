package servertest

import (
	"net/http"
	"reflect"
)

// ServerConfigAdapter maps explicit fixture fields to either app's configuration.
// Missing or incompatible fields fail immediately rather than skipping setup.
func ServerConfigAdapter[C, F any](newHandler func(C) http.Handler) func(F) http.Handler {
	return func(config F) http.Handler {
		var target C
		destination, source := reflect.ValueOf(&target).Elem(), reflect.ValueOf(config)
		for index := range source.NumField() {
			name := source.Type().Field(index).Name
			field := destination.FieldByName(name)
			if !field.IsValid() || !field.CanSet() || field.Type() != source.Field(index).Type() {
				panic("incompatible server fixture configuration field: " + name)
			}
			field.Set(source.Field(index))
		}
		return newHandler(target)
	}
}
