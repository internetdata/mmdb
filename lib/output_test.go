package lib

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/oschwald/maxminddb-golang/v2"
)

// TestCmdImport_FailedRebuildLeavesTheOldFile rebuilds a database a reader has
// open, as a server does, first with a bad row and then with good ones.
func TestCmdImport_FailedRebuildLeavesTheOldFile(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "db.mmdb")
	csvs := map[string]string{
		"old.csv":    "network,country\n1.0.0.0/24,AU\n",
		"new.csv":    "network,country\n1.0.0.0/24,NZ\n",
		"ragged.csv": "network,country\n1.0.0.0/24,NZ\n8.8.8.8,US,extra\n",
	}
	for name, rows := range csvs {
		writeTestFile(t, filepath.Join(dir, name), rows)
	}
	if err := importCSV(filepath.Join(dir, "old.csv"), db); err != nil {
		t.Fatalf("import failed: %s", err)
	}
	if err := os.Chmod(db, 0640); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	reader := openMMDB(t, db)

	if err := importCSV(filepath.Join(dir, "ragged.csv"), db); err == nil {
		t.Fatal("expected the ragged import to fail")
	}
	if after, err := os.ReadFile(db); err != nil || !bytes.Equal(before, after) {
		t.Errorf("the failed import changed the old file (%v)", err)
	}
	if got := country(t, reader, "1.0.0.1"); got != "AU" {
		t.Errorf("the reader answered %q, want AU", got)
	}
	assertDirHolds(t, dir, "db.mmdb", "new.csv", "old.csv", "ragged.csv")

	if err := importCSV(filepath.Join(dir, "new.csv"), db); err != nil {
		t.Fatalf("import failed: %s", err)
	}
	if got := country(t, reader, "1.0.0.1"); got != "AU" {
		t.Errorf("the reader answered %q, want AU from its copy", got)
	}
	if got := country(t, openMMDB(t, db), "1.0.0.1"); got != "NZ" {
		t.Errorf("the rebuilt file answered %q, want NZ", got)
	}
	if fi, err := os.Stat(db); err != nil || fi.Mode().Perm() != 0640 {
		t.Errorf("the rebuilt file's mode is %v (%v), want 0640", fi.Mode().Perm(), err)
	}
	assertDirHolds(t, dir, "db.mmdb", "new.csv", "old.csv", "ragged.csv")
}

func TestCmdCompress_ReplacesItsOutputWhole(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.mmdb")
	out := filepath.Join(dir, "out.mmdb")
	writeUnsharedMMDB(t, in, 32, true)
	writeUnsharedMMDB(t, out, 24, false)
	if err := os.Chmod(out, 0600); err != nil {
		t.Fatal(err)
	}
	reader := openMMDB(t, out)

	if err := CmdCompress(CmdCompressFlags{}, []string{in, out}, func() {}); err != nil {
		t.Fatalf("compress failed: %s", err)
	}
	if reader.Metadata.RecordSize != 24 || lookup(t, reader, "1.0.0.1") != "x" {
		t.Error("the reader of the old output lost its copy")
	}
	if got := openMMDB(t, out).Metadata.RecordSize; got != 32 {
		t.Errorf("the output's record size is %d, want the input's 32", got)
	}
	if fi, err := os.Stat(out); err != nil || fi.Mode().Perm() != 0600 {
		t.Errorf("the output's mode is %v (%v), want 0600", fi.Mode().Perm(), err)
	}
	assertDirHolds(t, dir, "in.mmdb", "out.mmdb")
}

func TestCreateOut_TakesAFreeName(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "db.mmdb")
	taken := fmt.Sprintf(".db.mmdb.%d-0.tmp", os.Getpid())
	writeTestFile(t, filepath.Join(dir, taken), "not ours")
	writeTestFile(t, filepath.Join(dir, "in.csv"), "network,country\n1.0.0.0/24,AU\n")

	if err := importCSV(filepath.Join(dir, "in.csv"), db); err != nil {
		t.Fatalf("import failed: %s", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, taken)); err != nil || string(b) != "not ours" {
		t.Errorf("%s holds %q (%v), want what it held", taken, b, err)
	}
	if got := country(t, openMMDB(t, db), "1.0.0.1"); got != "AU" {
		t.Errorf("the new file answered %q, want AU", got)
	}
	assertDirHolds(t, dir, taken, "db.mmdb", "in.csv")
}

// TestCreateOut_WritesDirectly covers outputs that a rename would replace
// differently from writing them, which are written directly.
func TestCreateOut_WritesDirectly(t *testing.T) {
	setup := func(t *testing.T) (string, string) {
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "old.csv"), "network,country\n1.0.0.0/24,AU\n")
		writeTestFile(t, filepath.Join(dir, "new.csv"), "network,country\n1.0.0.0/24,NZ\n")
		db := filepath.Join(dir, "db.mmdb")
		if err := importCSV(filepath.Join(dir, "old.csv"), db); err != nil {
			t.Fatalf("import failed: %s", err)
		}
		return dir, db
	}

	t.Run("a symlink", func(t *testing.T) {
		dir, db := setup(t)
		link := filepath.Join(dir, "link.mmdb")
		if err := os.Symlink("db.mmdb", link); err != nil {
			t.Skip(err)
		}
		if err := importCSV(filepath.Join(dir, "new.csv"), link); err != nil {
			t.Fatalf("import failed: %s", err)
		}
		if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("the symlink was replaced (%v)", err)
		}
		if got := country(t, openMMDB(t, db), "1.0.0.1"); got != "NZ" {
			t.Errorf("its target answered %q, want NZ", got)
		}
	})

	t.Run("a second name", func(t *testing.T) {
		dir, db := setup(t)
		other := filepath.Join(dir, "other.mmdb")
		if err := os.Link(db, other); err != nil {
			t.Skip(err)
		}
		if err := importCSV(filepath.Join(dir, "new.csv"), db); err != nil {
			t.Fatalf("import failed: %s", err)
		}
		if got := country(t, openMMDB(t, other), "1.0.0.1"); got != "NZ" {
			t.Errorf("the second name answered %q, want NZ", got)
		}
	})

	t.Run("a directory that refuses a new file", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root writes any directory")
		}
		dir, db := setup(t)
		if err := os.Chmod(dir, 0555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0755) })
		if err := importCSV(filepath.Join(dir, "new.csv"), db); err != nil {
			t.Fatalf("import failed: %s", err)
		}
		if got := country(t, openMMDB(t, db), "1.0.0.1"); got != "NZ" {
			t.Errorf("the file answered %q, want NZ", got)
		}
	})
}

func importCSV(in, out string) error {
	return CmdImport(CmdImportFlags{
		Ip: 6, Size: 32, Merge: "none", In: in, Out: out, Csv: true, NoNetwork: true, FieldsFromHdr: true,
	}, nil, func() {})
}

func country(t *testing.T, db *maxminddb.Reader, ip string) string {
	t.Helper()
	var record struct {
		Country string `maxminddb:"country"`
	}
	if err := db.Lookup(netip.MustParseAddr(ip)).Decode(&record); err != nil {
		t.Fatalf("lookup of %s failed: %s", ip, err)
	}
	return record.Country
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertDirHolds(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Errorf("%s holds %v, want %v", dir, names, want)
	}
}
