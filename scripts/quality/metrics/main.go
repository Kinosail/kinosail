// Command metrics owns Kinosail's deterministic source metric gates.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) == 2 && args[0] == "halstead" {
		report, err := halsteadFile(args[1])
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(report)
	}
	if len(args) >= 3 && len(args) <= 10000 && args[0] == "crap" {
		return checkCRAP(args[1], args[2:], output)
	}
	return errors.New("usage: metrics halstead FILE | metrics crap COVERAGE PACKAGE...")
}
