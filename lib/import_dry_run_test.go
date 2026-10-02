package lib

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxmind/mmdbwriter/v2"
	"github.com/oschwald/maxminddb-golang/v2"
)

// Each case is checked against a real build of the same input. None shares a
// subtree, so the built file's node count is the tree as built.
func TestDryRun_MatchesBuild(t *testing.T) {
	tests := []struct {
		name  string
		flags func(*CmdImportFlags)
		csv   string
		// slack is how far over the build's count the dry run may be: the
		// aliases' and reserved networks' paths, counted apart from the rows.
		slack uint64
	}{
		{
			name: "single IPs and CIDRs, every record distinct",
			csv:  "network,v\n1.0.0.1,a\n1.0.0.2,b\n10.0.0.0/8,c\n2001:db8::1,d\n2600::/16,e\n",
		},
		{
			name:  "neighbors with the same record merge",
			flags: func(f *CmdImportFlags) { f.NoNetwork = true },
			csv:   "network,v\n1.0.0.0/25,x\n1.0.0.128/25,x\n2.0.0.0/24,y\n",
		},
		{
			name:  "neighbors with different records do not",
			flags: func(f *CmdImportFlags) { f.NoNetwork = true },
			csv:   "network,v\n1.0.0.0/25,x\n1.0.0.128/25,z\n2.0.0.0/24,y\n",
		},
		{
			name:  "merges climb more than one level",
			flags: func(f *CmdImportFlags) { f.NoNetwork = true },
			csv:   "network,v\n1.0.0.0/26,x\n1.0.0.64/26,x\n1.0.0.128/25,x\n9.0.0.0/24,y\n",
		},
		{
			name:  "ranges",
			flags: func(f *CmdImportFlags) { f.NoNetwork = true },
			csv:   "start_ip,end_ip,v\n1.0.0.0,1.0.2.255,x\n1.0.3.0,1.0.3.255,y\n2001:db8::,2001:db8::ff,z\n",
		},
		{
			name:  "an IPv4 tree",
			flags: func(f *CmdImportFlags) { f.Ip = 4 },
			csv:   "network,v\n1.0.0.1,a\n8.8.8.0/24,b\n100.64.0.0/10,c\n",
		},
		{
			name: "aliases and reserved networks",
			flags: func(f *CmdImportFlags) {
				f.Alias6to4 = true
				f.DisallowReserved = true
			},
			csv:   "network,v\n1.1.1.1,a\n8.8.8.8,b\n2606:4700::1111,c\n",
			slack: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := CmdImportFlags{Ip: 6, Size: 32, Merge: "none", FieldsFromHdr: true}
			if tt.flags != nil {
				tt.flags(&f)
			}
			got := dryRunCSV(t, f, tt.csv)
			nodes, data := buildCSV(t, f, tt.csv)
			if got.Nodes < nodes || got.Nodes > nodes+tt.slack {
				t.Errorf("nodes: dry run counted %d, the build has %d", got.Nodes, nodes)
			}
			if got.DataBytes != data {
				t.Errorf("records: dry run says %d bytes, the build wrote %d", got.DataBytes, data)
			}
		})
	}
}

func TestDryRun_WritesNothing(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")
	out := filepath.Join(dir, "out.mmdb")
	if err := os.WriteFile(in, []byte("network,v\n1.0.0.0/24,a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := CmdImportFlags{Ip: 6, Size: 32, Merge: "none", In: in, Out: out, Csv: true, DryRun: true}

	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	err = CmdImport(f, []string{}, func() {})
	w.Close()
	os.Stdout = stdout
	printed, _ := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("dry run created %s", out)
	}
	want := "dry run for " + out + "; nothing was written\n  input:   1 rows, 1 networks, 1 distinct records\n"
	if !strings.HasPrefix(string(printed), want) {
		t.Errorf("printed:\n%s\nwant it to start:\n%s", printed, want)
	}
}

func TestDryRunReport_Fits(t *testing.T) {
	tests := []struct {
		nodes uint64
		data  int64
		size  int
		want  string
	}{
		{1000, 100, 32, "in 24-, 28- and 32-bit records; 32-bit at most 0.01% full"},
		{1_644_664, 10_148, 24, "in 24-, 28- and 32-bit records; 24-bit at most 10% full"},
		{20_000_000, 0, 28, "in 28- and 32-bit records; 28-bit at most 7% full"},
		{20_000_000, 0, 24, "in 28- and 32-bit records, not --size 24"},
		{3_480_000_000, 599_000_000, 32, "in 32-bit records only, at most 95% full"},
		{4_500_000_000, 0, 32, "may not: up to 105% of the 32-bit space"},
	}
	for _, tt := range tests {
		r := dryRunReport{Nodes: tt.nodes, DataBytes: tt.data}
		if got := r.fits(tt.size); got != tt.want {
			t.Errorf("fits(%d nodes, %d bytes, --size %d) = %q, want %q", tt.nodes, tt.data, tt.size, got, tt.want)
		}
	}
}

func TestHumanCountAndBytes(t *testing.T) {
	counts := map[uint64]string{
		999:           "999",
		909_573:       "909.6K",
		119_500_000:   "119.5M",
		3_480_000_000: "3.48B",
	}
	for n, want := range counts {
		if got := humanCount(n); got != want {
			t.Errorf("humanCount(%d) = %q, want %q", n, got, want)
		}
	}
	if got := humanBytes(5_954_767, 1000, "B", "kB", "MB", "GB", "TB"); got != "5.95 MB" {
		t.Errorf("humanBytes = %q, want 5.95 MB", got)
	}
	if got := humanBytes(159_500_000_000, 1024, "B", "KiB", "MiB", "GiB", "TiB"); got != "149 GiB" {
		t.Errorf("humanBytes = %q, want 149 GiB", got)
	}
}

// dryRunCSV feeds csvData to a dry run the way CmdImport does.
func dryRunCSV(t *testing.T, f CmdImportFlags, csvData string) dryRunReport {
	t.Helper()
	d, err := newDryRun(mmdbwriter.Options{
		IPVersion:               f.Ip,
		RecordSize:              f.Size,
		DisableIPv4Aliasing:     !f.Alias6to4,
		IncludeReservedNetworks: !f.DisallowReserved,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(bufio.NewReader(strings.NewReader(csvData)))
	dataColStart, rows := 1, 0
	for i := 0; ; i++ {
		parts, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			ParseCSVHeaders(parts, &f, &dataColStart)
			continue
		}
		if err := AppendCSVRecord(f, dataColStart, ',', parts, d); err != nil {
			t.Fatal(err)
		}
		rows++
	}
	report, err := d.report(rows)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// buildCSV imports csvData for real, and returns the file's node count and
// the size of its data section.
func buildCSV(t *testing.T, f CmdImportFlags, csvData string) (uint64, int64) {
	t.Helper()
	dir := t.TempDir()
	f.In = filepath.Join(dir, "in.csv")
	f.Out = filepath.Join(dir, "out.mmdb")
	f.Csv = true
	if err := os.WriteFile(f.In, []byte(csvData), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CmdImport(f, []string{}, func() {}); err != nil {
		t.Fatal(err)
	}
	db, err := maxminddb.Open(f.Out)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	b, err := os.ReadFile(f.Out)
	if err != nil {
		t.Fatal(err)
	}
	nodes := uint64(db.Metadata.NodeCount)
	tree := int64(nodes) * int64(db.Metadata.RecordSize) / 4
	return nodes, int64(bytes.LastIndex(b, metadataMarker)) - tree - 16
}
