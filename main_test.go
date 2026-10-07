package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

func TestExitStatus(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFile(t, dir, "a.csv", "network,country\n1.0.0.0/24,AU\n8.8.8.8,US\n")
	ragged := writeFile(t, dir, "ragged.csv", "network,country\n1.0.0.0/24,AU\n8.8.8.8,US,extra\n")
	db := filepath.Join(dir, "a.mmdb")
	if _, stderr, code := runMain(t, "import", "--csv", csvPath, db); code != 0 {
		t.Fatalf("import exited %d: %s", code, stderr)
	}

	// The first node's two records point past the end of the file.
	b, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	copy(b, bytes.Repeat([]byte{0xff}, 8))
	corrupt := writeFile(t, dir, "corrupt.mmdb", string(b))

	tests := []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{"import a ragged csv", []string{"import", "--csv", ragged, filepath.Join(dir, "r.mmdb")},
			1, "", "err: input scanning failed: "},
		{"import a missing file",
			[]string{"import", "--csv", filepath.Join(dir, "no.csv"), filepath.Join(dir, "n.mmdb")},
			1, "", "err: invalid input file "},
		{"verify a valid file", []string{"verify", db}, 0, "valid\n", ""},
		{"verify a corrupt file", []string{"verify", corrupt}, 1, "invalid: ", ""},
		{"verify a missing file", []string{"verify", filepath.Join(dir, "no.mmdb")},
			1, "", "err: couldn't open mmdb file: "},
		{"read an address the file lacks", []string{"read", "9.9.9.9", db},
			0, "", "err: couldn't get data for 9.9.9.9\n"},
		{"completion of an unknown shell", []string{"completion", "bogus"},
			1, "err: bogus is not a valid subcommand\n", ""},
		{"an unknown flag", []string{"verify", "--bogus", db}, 2, "", "Usage of "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runMain(t, tc.args...)
			if code != tc.code {
				t.Errorf("exit %d, want %d; stderr: %s", code, tc.code, stderr)
			}
			if !strings.HasPrefix(stdout, tc.stdout) || (tc.stdout == "" && stdout != "") {
				t.Errorf("stdout %q, want it to start with %q", stdout, tc.stdout)
			}
			if !strings.HasPrefix(stderr, tc.stderr) || (tc.stderr == "" && stderr != "") {
				t.Errorf("stderr %q, want it to start with %q", stderr, tc.stderr)
			}
		})
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

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
