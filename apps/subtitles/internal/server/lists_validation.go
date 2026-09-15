package server

import "github.com/MikeO7/kinosail/packages/catalog"

func validListName(name string) bool {
	return catalog.ValidListName(name)
}
