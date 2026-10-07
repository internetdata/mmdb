package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestMain runs main itself when a test re-runs this binary, so a test sees
// the exit status and output a caller of mmdb sees.
func TestMain(m *testing.M) {
	if os.Getenv("MMDB_TEST_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestReadWithNoFilePrintsHelp(t *testing.T) {
	for _, args := range [][]string{{"read", "--nocolor"}, {"read", "-f", "csv"}} {
		stdout, stderr, code := runMain(t, args...)
		if code != 0 || !strings.HasPrefix(stdout, "Usage: ") || stderr != "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want its help", args, code, stdout, stderr)
		}
	}
}

func runMain(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, args...)
	cmd.Env = append(os.Environ(), "MMDB_TEST_RUN_MAIN=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), 0
}
