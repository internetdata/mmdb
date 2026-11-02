package lib

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCmdDiff_Sorted diffs two small databases, with the caches kept and with
// them starting over at every record, and checks the changes print sorted and
// each once.
func TestCmdDiff_Sorted(t *testing.T) {
	tempDir := t.TempDir()
	inputs := map[string]string{
		"old": "network,v\n1.0.0.0/24,a\n2.0.0.0/23,b\n5.0.0.0/24,g\n5.0.1.0/24,h\n" +
			"8.8.8.0/24,c\n9.9.9.0/24,d\n2001:db8::/32,e\n",
		"new": "network,v\n1.0.0.0/24,a\n2.0.0.0/24,b\n2.0.1.0/24,b2\n3.0.0.0/24,f\n5.0.0.0/23,g\n" +
			"8.8.8.0/24,C\n9.9.9.0/24,d\n2001:db8::/32,E\n",
	}
	for name, csv := range inputs {
		in := filepath.Join(tempDir, name+".csv")
		if err := os.WriteFile(in, []byte(csv), 0644); err != nil {
			t.Fatal(err)
		}
		err := CmdImport(CmdImportFlags{
			Ip:        6,
			Merge:     "none",
			NoNetwork: true,
			In:        in,
			Out:       filepath.Join(tempDir, name+".mmdb"),
			Csv:       true,
		}, []string{}, func() {})
		if err != nil {
			t.Fatalf("import %s failed: %s", name, err)
		}
	}
	args := []string{filepath.Join(tempDir, "old.mmdb"), filepath.Join(tempDir, "new.mmdb")}

	// The split's 2.0.0.0/23 -> 2.0.0.0/24 and the merge's 5.0.0.0/24 ->
	// 5.0.0.0/23 are each seen from both files, and printed and counted once.
	want := `** SUBNETS **
2.0.0.0/23 -> 2.0.0.0/24
2.0.0.0/23 -> 2.0.1.0/24
3.0.0.0/8 -> 3.0.0.0/24
5.0.0.0/24 -> 5.0.0.0/23
5.0.1.0/24 -> 5.0.0.0/23
5 subnet(s) modified.

** RECORDS **
8.8.8.0/24
	-map[v:c]
	+map[v:C]
2001:db8::/32
	-map[v:e]
	+map[v:E]
2 record(s) modified.
`
	defer func(c int) { diffCacheCap = c }(diffCacheCap)
	for _, cacheCap := range []int{diffCacheCap, 1} {
		diffCacheCap = cacheCap
		for range 3 {
			got := diffStdout(t, CmdDiffFlags{Subnets: true, Records: true}, args)
			if got != want {
				t.Fatalf("cache cap %d: got\n%s\nwant\n%s", cacheCap, got, want)
			}
		}
	}

	// With neither flag, only the counts print, the same ones.
	got := diffStdout(t, CmdDiffFlags{}, args)
	if want := "5 subnet(s) modified.\n2 record(s) modified.\n"; got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func diffStdout(t *testing.T, f CmdDiffFlags, args []string) string {
	t.Helper()
	out, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	stdout := os.Stdout
	os.Stdout = out
	err = CmdDiff(f, args, func() {})
	os.Stdout = stdout
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	printed, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(printed)
}
