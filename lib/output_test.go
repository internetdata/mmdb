package lib

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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

// TestCmdImport_Backup rebuilds with --backup: a failed rebuild keeps nothing,
// and each rebuild keeps the very file it replaces, until the next one.
func TestCmdImport_Backup(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "db.mmdb")
	csvs := map[string]string{
		"au.csv":     "network,country\n1.0.0.0/24,AU\n",
		"nz.csv":     "network,country\n1.0.0.0/24,NZ\n",
		"fj.csv":     "network,country\n1.0.0.0/24,FJ\n",
		"ragged.csv": "network,country\n1.0.0.0/24,NZ\n8.8.8.8,US,extra\n",
	}
	for name, rows := range csvs {
		writeTestFile(t, filepath.Join(dir, name), rows)
	}
	rebuild := func(in string) error {
		f := importFlags(filepath.Join(dir, in), db)
		f.Backup = ".bak"
		return CmdImport(f, nil, func() {})
	}

	if err := rebuild("au.csv"); err != nil {
		t.Fatalf("import failed: %s", err)
	}
	assertDirHolds(t, dir, "au.csv", "db.mmdb", "fj.csv", "nz.csv", "ragged.csv")
	if err := os.Chmod(db, 0640); err != nil {
		t.Fatal(err)
	}
	old, err := os.Stat(db)
	if err != nil {
		t.Fatal(err)
	}
	reader := openMMDB(t, db)

	if err := rebuild("ragged.csv"); err == nil {
		t.Fatal("expected the ragged import to fail")
	}
	assertDirHolds(t, dir, "au.csv", "db.mmdb", "fj.csv", "nz.csv", "ragged.csv")

	if err := rebuild("nz.csv"); err != nil {
		t.Fatalf("import failed: %s", err)
	}
	kept, err := os.Stat(db + ".bak")
	if err != nil || !os.SameFile(old, kept) || kept.Mode().Perm() != 0640 {
		t.Errorf("the backup is not the old file with its mode (%v)", err)
	}
	if got := country(t, reader, "1.0.0.1"); got != "AU" {
		t.Errorf("the reader answered %q, want AU from its copy", got)
	}
	assertAnswers(t, db, "NZ")
	assertAnswers(t, db+".bak", "AU")

	if err := rebuild("ragged.csv"); err == nil {
		t.Fatal("expected the ragged import to fail")
	}
	assertAnswers(t, db, "NZ")
	assertAnswers(t, db+".bak", "AU")

	if err := rebuild("fj.csv"); err != nil {
		t.Fatalf("import failed: %s", err)
	}
	assertAnswers(t, db, "FJ")
	assertAnswers(t, db+".bak", "NZ")
	assertDirHolds(t, dir, "au.csv", "db.mmdb", "db.mmdb.bak", "fj.csv", "nz.csv", "ragged.csv")
}

// TestCmdImport_BackupChecks covers what refuses a backup, each before any
// input is read: the error comes ahead of a missing input's.
func TestCmdImport_BackupChecks(t *testing.T) {
	tests := []struct {
		name   string
		suffix string
		in     string
		setup  func(db string) error
		want   string
	}{
		{"a directory", "~", "nosuch.csv", func(db string) error {
			return os.Mkdir(db+"~", 0755)
		}, "which is a directory"},
		{"another name for the output", ".old", "nosuch.csv", func(db string) error {
			return os.Link(db, db+".old")
		}, "which is another name for"},
		{"the input", ".csv", "db.mmdb.csv", func(db string) error {
			return os.WriteFile(db+".csv", []byte("network,country\n1.0.0.0/24,NZ\n"), 0644)
		}, "which is the input"},
		{"a path separator", "/x", "nosuch.csv", func(string) error {
			return nil
		}, "must not contain a path separator"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			db := filepath.Join(dir, "db.mmdb")
			writeTestFile(t, filepath.Join(dir, "au.csv"), "network,country\n1.0.0.0/24,AU\n")
			if err := importCSV(filepath.Join(dir, "au.csv"), db); err != nil {
				t.Fatalf("import failed: %s", err)
			}
			if err := tc.setup(db); err != nil {
				t.Skip(err)
			}
			f := importFlags(filepath.Join(dir, tc.in), db)
			f.Backup = tc.suffix
			if err := CmdImport(f, nil, func() {}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want an error with %q", err, tc.want)
			}
			assertAnswers(t, db, "AU")
		})
	}

	t.Run("no output", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "au.csv"), "network,country\n1.0.0.0/24,AU\n")
		f := importFlags(filepath.Join(dir, "au.csv"), "")
		f.Backup = ".bak"
		if err := CmdImport(f, nil, func() {}); err == nil || err.Error() != "--backup needs --out" {
			t.Errorf("got %v, want --backup needs --out", err)
		}
	})

	t.Run("nothing to back up", func(t *testing.T) {
		dir := t.TempDir()
		db := filepath.Join(dir, "db.mmdb")
		writeTestFile(t, filepath.Join(dir, "au.csv"), "network,country\n1.0.0.0/24,AU\n")
		if err := os.Mkdir(db+"~", 0755); err != nil {
			t.Fatal(err)
		}
		f := importFlags(filepath.Join(dir, "au.csv"), db)
		f.Backup = "~"
		if err := CmdImport(f, nil, func() {}); err != nil {
			t.Fatalf("import failed: %s", err)
		}
		assertAnswers(t, db, "AU")
	})
}

func TestCmdCompress_Backup(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.mmdb")
	out := filepath.Join(dir, "out.mmdb")
	writeUnsharedMMDB(t, in, 32, true)
	writeUnsharedMMDB(t, out, 24, false)

	if err := CmdCompress(CmdCompressFlags{Backup: "~"}, []string{in, out}, func() {}); err != nil {
		t.Fatalf("compress failed: %s", err)
	}
	if got := openMMDB(t, out+"~").Metadata.RecordSize; got != 24 {
		t.Errorf("the backup's record size is %d, want the old output's 24", got)
	}
	if got := openMMDB(t, out).Metadata.RecordSize; got != 32 {
		t.Errorf("the output's record size is %d, want the input's 32", got)
	}
	assertDirHolds(t, dir, "in.mmdb", "out.mmdb", "out.mmdb~")
}

// TestCreateOut_BackupWhenWritingDirectly covers outputs written in place,
// whose backup is a copy made before the write.
func TestCreateOut_BackupWhenWritingDirectly(t *testing.T) {
	setup := func(t *testing.T) (string, string) {
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "old.csv"), "network,country\n1.0.0.0/24,AU\n")
		writeTestFile(t, filepath.Join(dir, "new.csv"), "network,country\n1.0.0.0/24,NZ\n")
		writeTestFile(t, filepath.Join(dir, "ragged.csv"), "network,country\n8.8.8.8,US,extra\n")
		db := filepath.Join(dir, "db.mmdb")
		if err := importCSV(filepath.Join(dir, "old.csv"), db); err != nil {
			t.Fatalf("import failed: %s", err)
		}
		return dir, db
	}
	rebuild := func(in, out string) error {
		f := importFlags(in, out)
		f.Backup = "~"
		return CmdImport(f, nil, func() {})
	}

	t.Run("a symlink", func(t *testing.T) {
		dir, db := setup(t)
		link := filepath.Join(dir, "link.mmdb")
		if err := os.Symlink("db.mmdb", link); err != nil {
			t.Skip(err)
		}
		if err := rebuild(filepath.Join(dir, "new.csv"), link); err != nil {
			t.Fatalf("import failed: %s", err)
		}
		if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("the symlink was replaced (%v)", err)
		}
		assertAnswers(t, db, "NZ")
		assertAnswers(t, link+"~", "AU")
		assertDirHolds(t, dir, "db.mmdb", "link.mmdb", "link.mmdb~", "new.csv", "old.csv", "ragged.csv")
	})

	t.Run("a second name", func(t *testing.T) {
		dir, db := setup(t)
		other := filepath.Join(dir, "other.mmdb")
		if err := os.Link(db, other); err != nil {
			t.Skip(err)
		}
		if err := rebuild(filepath.Join(dir, "new.csv"), db); err != nil {
			t.Fatalf("import failed: %s", err)
		}
		assertAnswers(t, other, "NZ")
		assertAnswers(t, db+"~", "AU")
	})

	t.Run("a failed write", func(t *testing.T) {
		dir, _ := setup(t)
		link := filepath.Join(dir, "link.mmdb")
		if err := os.Symlink("db.mmdb", link); err != nil {
			t.Skip(err)
		}
		if err := rebuild(filepath.Join(dir, "ragged.csv"), link); err == nil {
			t.Fatal("expected the ragged import to fail")
		}
		assertAnswers(t, link+"~", "AU")
	})

	t.Run("a directory that refuses a new file", func(t *testing.T) {
		if os.Geteuid() == 0 || runtime.GOOS == "windows" {
			t.Skip("root writes any directory, and Windows ignores its mode")
		}
		dir, db := setup(t)
		if err := os.Chmod(dir, 0555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0755) })
		if err := rebuild(filepath.Join(dir, "new.csv"), db); err == nil {
			t.Fatal("expected the import to fail, with nowhere to keep the backup")
		}
		assertAnswers(t, db, "AU")
	})
}

func importCSV(in, out string) error {
	return CmdImport(importFlags(in, out), nil, func() {})
}

func importFlags(in, out string) CmdImportFlags {
	return CmdImportFlags{
		Ip: 6, Size: 32, Merge: "none", In: in, Out: out, Csv: true, NoNetwork: true, FieldsFromHdr: true,
	}
}

// assertAnswers checks what the database at path holds for 1.0.0.1.
func assertAnswers(t *testing.T, path, want string) {
	t.Helper()
	if got := country(t, openMMDB(t, path), "1.0.0.1"); got != want {
		t.Errorf("%s answered %q, want %q", filepath.Base(path), got, want)
	}
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
