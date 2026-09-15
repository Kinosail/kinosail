package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/MikeO7/kinosail/packages/mcpgateway"
	"github.com/MikeO7/kinosail/packages/routeinventory"
	"github.com/MikeO7/kinosail/packages/scim"
)

var exit = os.Exit

func main() {
	exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: routeinventory APP_DIR PACKAGES_DIR")
		return 2
	}
	routes, err := routeinventory.Discover(args[0], args[1], scim.Patterns(), mcpgateway.Patterns())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(routes); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
