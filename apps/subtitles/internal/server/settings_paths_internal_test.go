package server

import "strings"

func within(parent, child string) bool {
	return parent == "." || child == parent || strings.HasPrefix(child, parent+"/")
}
