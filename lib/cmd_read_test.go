// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

import (
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oschwald/maxminddb-golang/v2"
)

// TestReadLookupOnIPv6Database verifies that both IPv4 and IPv6 lookups work
// on IPv6 databases. The IPv4 cases are a regression test: netip.AddrFromSlice
// on a 16-byte net.IP (as returned by net.ParseIP for IPv4) produces an
// IPv4-mapped-IPv6 address (::ffff:x.x.x.x) which maxminddb-golang/v2 does
// not resolve through the IPv4 subtree. The fix is to call .Unmap() before
// lookup.
func TestReadLookupOnIPv6Database(t *testing.T) {
	// Create a small IPv6 MMDB with IPv4 data via import.
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.csv")
	mmdbFile := filepath.Join(tempDir, "test.mmdb")

	csvData := "network,descr\n127.0.0.0/8,Loopback\n192.168.0.0/16,Private-Use\n2001:db8::/32,Documentation\nfc00::/7,Unique-Local\n"
	if err := os.WriteFile(inputFile, []byte(csvData), 0644); err != nil {
		t.Fatal(err)
	}

	err := CmdImport(CmdImportFlags{
		Ip:    6,
		Size:  32,
		Merge: "none",
		In:    inputFile,
		Out:   mmdbFile,
		Csv:   true,
	}, []string{}, func() {})
	if err != nil {
		t.Fatalf("import failed: %s", err)
	}

	db, err := maxminddb.Open(mmdbFile)
	if err != nil {
		t.Fatalf("failed to open MMDB: %s", err)
	}
	defer db.Close()

	if db.Metadata.IPVersion != 6 {
		t.Fatalf("expected IPv6 database, got version %d", db.Metadata.IPVersion)
	}

	tests := []struct {
		ip       string
		expected string
	}{
		{"127.0.0.1", "Loopback"},
		{"192.168.1.1", "Private-Use"},
		{"2001:db8::1", "Documentation"},
		{"fd00::1", "Unique-Local"},
	}

	for _, tc := range tests {
		t.Run(tc.ip, func(t *testing.T) {
			// net.ParseIP returns a 16-byte slice for IPv4, same as
			// iputil.IPListFromAllSrcs. This is the code path that
			// CmdRead uses.
			ip := net.ParseIP(tc.ip)
			if len(ip) != 16 {
				t.Fatalf("expected 16-byte IP, got %d", len(ip))
			}

			addr, ok := netip.AddrFromSlice(ip)
			if !ok {
				t.Fatalf("AddrFromSlice failed for %s", tc.ip)
			}
			addr = addr.Unmap()

			var record map[string]interface{}
			if err := db.Lookup(addr).Decode(&record); err != nil {
				t.Fatalf("lookup failed: %s", err)
			}
			if len(record) == 0 {
				t.Fatalf("empty record for %s", tc.ip)
			}
			if got := record["descr"]; got != tc.expected {
				t.Errorf("got descr=%v, want %s", got, tc.expected)
			}
		})
	}
}

// TestCmdRead_CachedRecords reads each record again and again, with the cache
// kept and starting over at every record, in every format.
func TestCmdRead_CachedRecords(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.json")
	mmdbFile := filepath.Join(tempDir, "test.mmdb")

	jsonData := `{"range": "1.1.1.0/24", "asn": "AS1", "ip": "theirs", "isp": "<A&B>", "zz": {"k": [1, 2]}}
{"range": "8.8.8.0/24", "asn": "AS2", "ip": "x", "isp": "G", "zz": "z"}
{"range": "9.9.9.0/24"}
`
	if err := os.WriteFile(inputFile, []byte(jsonData), 0644); err != nil {
		t.Fatal(err)
	}
	err := CmdImport(CmdImportFlags{
		Ip:        6,
		Merge:     "none",
		NoNetwork: true,
		In:        inputFile,
		Out:       mmdbFile,
		Json:      true,
	}, []string{}, func() {})
	if err != nil {
		t.Fatalf("import failed: %s", err)
	}

	// 9.9.9.9's record is empty and 7.7.7.7 has none: neither prints.
	addrs := []string{"1.1.1.1", "8.8.8.8", "1.1.1.2", "9.9.9.9", "7.7.7.7", "8.8.8.9", "1.1.1.3"}
	jsonLines := map[string]string{
		"1": `{"asn":"AS1","ip":"%s","isp":"\u003cA\u0026B\u003e","zz":{"k":[1,2]}}`,
		"8": `{"asn":"AS2","ip":"%s","isp":"G","zz":"z"}`,
	}
	var wantJSON, wantPretty strings.Builder
	for _, a := range addrs {
		tmpl, ok := jsonLines[a[:1]]
		if !ok {
			continue
		}
		wantJSON.WriteString(strings.Replace(tmpl, "%s", a, 1) + "\n")
	}
	for _, a := range addrs {
		switch a[:1] {
		case "1":
			wantPretty.WriteString("{\n  \"asn\": \"AS1\",\n  \"ip\": \"" + a +
				"\",\n  \"isp\": \"\\u003cA\\u0026B\\u003e\",\n  \"zz\": {\n    \"k\": [\n" +
				"      1,\n      2\n    ]\n  }\n}\n")
		case "8":
			wantPretty.WriteString("{\n  \"asn\": \"AS2\",\n  \"ip\": \"" + a +
				"\",\n  \"isp\": \"G\",\n  \"zz\": \"z\"\n}\n")
		}
	}
	wantCSV := `ip,asn,ip,isp,zz
1.1.1.1,AS1,theirs,<A&B>,"{""k"":[1,2]}"
8.8.8.8,AS2,x,G,z
1.1.1.2,AS1,theirs,<A&B>,"{""k"":[1,2]}"
8.8.8.9,AS2,x,G,z
1.1.1.3,AS1,theirs,<A&B>,"{""k"":[1,2]}"
`
	tests := []struct {
		format string
		want   string
	}{
		{"json", wantJSON.String()},
		{"json-pretty", wantPretty.String()},
		{"csv", wantCSV},
		{"tsv", strings.NewReplacer(",", "\t", `"{""k"":[1,2]}"`, `{"k":[1,2]}`).Replace(wantCSV)},
	}

	for _, cacheCap := range []int{readCacheCap, 1} {
		for _, tc := range tests {
			t.Run(tc.format, func(t *testing.T) {
				defer func(c int) { readCacheCap = c }(readCacheCap)
				readCacheCap = cacheCap

				args := append(append([]string{}, addrs...), mmdbFile)
				got := captureStdout(t, func() error {
					return CmdRead(CmdReadFlags{Format: tc.format}, args, func() {})
				})
				if got != tc.want {
					t.Errorf("cache cap %d: got\n%s\nwant\n%s", cacheCap, got, tc.want)
				}
			})
		}
	}
}

// captureStdout runs fn with os.Stdout sent to a file, and returns what it wrote.
func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	out, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	stdout := os.Stdout
	os.Stdout = out
	err = fn()
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
