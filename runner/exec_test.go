package runner

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestHelperProcess is not a real test: Exec tests re-run the test binary with
// BRB_HELPER set, so they need no real git and work on every OS.
func TestHelperProcess(t *testing.T) {
	switch os.Getenv("BRB_HELPER") {
	case "":
		return
	case "stdout":
		fmt.Print("hello\r\n")
	case "indented":
		fmt.Print(" M a.go\n")
	case "fail":
		fmt.Fprint(os.Stderr, "bad thing")
		os.Exit(3)
	}

	os.Exit(0)
}

func helper(t *testing.T, mode string) (string, error) {
	t.Helper()
	t.Setenv("BRB_HELPER", mode)

	return Exec{}.Run(os.Args[0], "-test.run=TestHelperProcess")
}

func TestExec_ReturnsTrimmedStdout(t *testing.T) {
	out, err := helper(t, "stdout")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out != "hello" {
		t.Errorf("out = %q, want %q", out, "hello")
	}
}

func TestExec_KeepsLeadingWhitespace(t *testing.T) {
	out, err := helper(t, "indented")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out != " M a.go" {
		t.Errorf("out = %q, want %q", out, " M a.go")
	}
}

func TestExec_NonZeroExitIncludesStderr(t *testing.T) {
	_, err := helper(t, "fail")
	if err == nil || !strings.Contains(err.Error(), "bad thing") {
		t.Errorf("err = %v, want it to contain stderr %q", err, "bad thing")
	}
}

func TestExec_MissingBinary(t *testing.T) {
	_, err := Exec{}.Run("brb-no-such-binary")
	if err == nil {
		t.Error("want error for missing binary")
	}
}
