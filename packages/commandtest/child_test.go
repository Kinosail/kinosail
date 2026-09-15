package commandtest

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestCoveredChildUsesExactTargetAndPropagatesCoverage(t *testing.T) {
	command := CoveredChild(t, "Test.Exact[Target]", "KINOSAIL_CHILD_VALUE=override")
	if command.Args[1] != `-test.run=^Test\.Exact\[Target\]$` {
		t.Fatalf("child target is not exact: %v", command.Args)
	}
	if directory := flag.Lookup("test.gocoverdir"); directory != nil && directory.Value.String() != "" {
		if !slices.Contains(command.Args, "-test.gocoverdir="+directory.Value.String()) || !slices.Contains(command.Env, "GOCOVERDIR="+directory.Value.String()) {
			t.Fatal("child coverage directory not propagated to flags and environment")
		}
	}
}

func TestCoveredChildEnvironmentOverridesInheritedValue(t *testing.T) {
	t.Setenv("KINOSAIL_CHILD_VALUE", "inherited")
	command := CoveredChild(t, "TestCoveredChildProcess", "KINOSAIL_CHILD_VALUE=override", "KINOSAIL_COMMANDTEST_CHILD=1")
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "child value=override") {
		t.Fatalf("child environment=%q, %v", output, err)
	}
}

func TestCoveredChildProcess(t *testing.T) {
	if os.Getenv("KINOSAIL_COMMANDTEST_CHILD") != "1" {
		return
	}
	fmt.Println("child value=" + os.Getenv("KINOSAIL_CHILD_VALUE"))
}
