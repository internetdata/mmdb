// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oschwald/maxminddb-golang/v2"
)

// verifyMMDBContent is a test helper that verifies MMDB file contains expected entries
func verifyMMDBContent(t *testing.T, mmdbPath string, testCases []struct {
	ip       string
	expected map[string]interface{}
}) {
	t.Helper()

	// Check if output file was created
	if _, err := os.Stat(mmdbPath); os.IsNotExist(err) {
		t.Error("expected output file to be created")
	}

	db, err := maxminddb.Open(mmdbPath)
	if err != nil {
		t.Fatalf("failed to open MMDB file %s: %s", mmdbPath, err.Error())
	}
	defer db.Close()

	for _, tc := range testCases {
		addr, err := netip.ParseAddr(tc.ip)
		if err != nil {
			t.Errorf("failed to parse IP: %s", tc.ip)
			continue
		}

		var record map[string]interface{}
		err = db.Lookup(addr).Decode(&record)
		if err != nil {
			t.Errorf("failed to lookup IP %s: %s", tc.ip, err.Error())
			continue
		}

		if len(record) == 0 {
			t.Errorf("no record found for IP %s", tc.ip)
			continue
		}

		// Verify expected fields
		for key, expectedValue := range tc.expected {
			if actualValue, ok := record[key]; !ok {
				t.Errorf("missing field %s for IP %s", key, tc.ip)
			} else if actualValue != expectedValue {
				t.Errorf("incorrect value for field %s, IP %s: expected %v, got %v",
					key, tc.ip, expectedValue, actualValue)
			}
		}
	}
}

func TestCmdImportFlags_Init(t *testing.T) {
	// Test that default values are correctly defined
	// Note: We don't call Init() here because it registers global flags
	// which would cause conflicts when running multiple tests
	if CmdImportFlagsDefaults.Ip != 6 {
		t.Errorf("expected default Ip to be 6, got %d", CmdImportFlagsDefaults.Ip)
	}
	if CmdImportFlagsDefaults.Size != 0 {
		t.Errorf("expected default Size to be 0 (auto), got %d", CmdImportFlagsDefaults.Size)
	}
	if CmdImportFlagsDefaults.Merge != "none" {
		t.Errorf("expected default Merge to be 'none', got %s", CmdImportFlagsDefaults.Merge)
	}
	if CmdImportFlagsDefaults.Help != false {
		t.Errorf("expected default Help to be false, got %v", CmdImportFlagsDefaults.Help)
	}
	if CmdImportFlagsDefaults.DisableMetadataPtrs != true {
		t.Errorf("expected default DisableMetadataPtrs to be true, got %v", CmdImportFlagsDefaults.DisableMetadataPtrs)
	}
}

func TestCmdImport_InvalidIPVersion(t *testing.T) {
	f := CmdImportFlags{
		Ip:   3, // invalid
		Size: 32,
		In:   "test.csv",
		Out:  "test.mmdb",
	}

	err := CmdImport(f, []string{}, func() {})
	if err == nil {
		t.Error("expected error for invalid IP version")
	}
	if !strings.Contains(err.Error(), "ip version must be") {
		t.Errorf("expected IP version error, got: %s", err.Error())
	}
}

func TestCmdImport_InvalidRecordSize(t *testing.T) {
	f := CmdImportFlags{
		Ip:   6,
		Size: 16, // invalid
		In:   "test.csv",
		Out:  "test.mmdb",
	}

	err := CmdImport(f, []string{}, func() {})
	if err == nil {
		t.Error("expected error for invalid record size")
	}
	if !strings.Contains(err.Error(), "record size must be") {
		t.Errorf("expected record size error, got: %s", err.Error())
	}
}

func TestCmdImport_RecordSize(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")
	if err := os.WriteFile(in, []byte("network,country\n1.0.0.0/24,AU\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for size, want := range map[int]uint{0: 24, 24: 24, 28: 28, 32: 32} {
		out := filepath.Join(dir, fmt.Sprintf("out%d.mmdb", size))
		f := CmdImportFlags{Ip: 6, Size: size, Merge: "none", In: in, Out: out, Csv: true}
		if err := CmdImport(f, nil, func() {}); err != nil {
			t.Fatalf("--size %d: %s", size, err)
		}
		db, err := maxminddb.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		if db.Metadata.RecordSize != want {
			t.Errorf("--size %d wrote %d-bit records, want %d", size, db.Metadata.RecordSize, want)
		}
		db.Close()
	}
}

func TestRecordSizeFlag(t *testing.T) {
	for in, want := range map[string]int{"auto": 0, "24": 24, "28": 28, "32": 32, "16": 16} {
		var size int
		f := (*recordSizeFlag)(&size)
		if err := f.Set(in); err != nil || size != want {
			t.Errorf("Set(%q) = %d, %v; want %d", in, size, err, want)
		}
		if f.String() != in {
			t.Errorf("Set(%q) prints as %q", in, f.String())
		}
	}
	var size int
	if err := (*recordSizeFlag)(&size).Set("big"); err == nil {
		t.Error("Set(\"big\") took a size that is no number")
	}
}

func TestCmdImport_InvalidMergeStrategy(t *testing.T) {
	f := CmdImportFlags{
		Ip:    6,
		Size:  32,
		Merge: "invalid", // invalid
		In:    "test.csv",
		Out:   "test.mmdb",
	}

	err := CmdImport(f, []string{}, func() {})
	if err == nil {
		t.Error("expected error for invalid merge strategy")
	}
	if !strings.Contains(err.Error(), "merge strategy must be") {
		t.Errorf("expected merge strategy error, got: %s", err.Error())
	}
}

func TestCmdImport_MultipleFileTypes(t *testing.T) {
	f := CmdImportFlags{
		Ip:    6,
		Size:  32,
		Merge: "none",
		Csv:   true,
		Tsv:   true, // conflicting with CSV
		In:    "test.csv",
		Out:   "test.mmdb",
	}

	err := CmdImport(f, []string{}, func() {})
	if err == nil {
		t.Error("expected error for multiple file types")
	}
	if !strings.Contains(err.Error(), "multiple input file types") {
		t.Errorf("expected multiple file types error, got: %s", err.Error())
	}
}

func TestCmdImport_ConflictingFieldSources(t *testing.T) {
	f := CmdImportFlags{
		Ip:            6,
		Size:          32,
		Merge:         "none",
		Fields:        []string{"field1"},
		FieldsFromHdr: true, // conflicting with Fields
		NoFields:      true, // conflicting with both
		In:            "test.csv",
		Out:           "test.mmdb",
	}

	err := CmdImport(f, []string{}, func() {})
	if err == nil {
		t.Error("expected error for conflicting field sources")
	}
	if !strings.Contains(err.Error(), "conflicting field sources") {
		t.Errorf("expected conflicting field sources error, got: %s", err.Error())
	}
}

func TestCmdImport_EmptyInput(t *testing.T) {
	tempDir := t.TempDir()
	emptyFile := filepath.Join(tempDir, "empty.csv")
	outputFile := filepath.Join(tempDir, "output.mmdb")

	// Create empty input file
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	f := CmdImportFlags{
		Ip:    6,
		Size:  32,
		Merge: "none",
		In:    emptyFile,
		Out:   outputFile,
		Csv:   true,
	}

	err := CmdImport(f, []string{}, func() {})
	if err == nil {
		t.Error("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "nothing to import") {
		t.Errorf("expected 'nothing to import' error, got: %s", err.Error())
	}
}

func TestCmdImport_SimpleCSV(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.csv")
	outputFile := filepath.Join(tempDir, "output.mmdb")

	csvData := "network,country\n167.153.128.0/17,US\n204.138.232.0/24,CA\n"
	if err := os.WriteFile(inputFile, []byte(csvData), 0644); err != nil {
		t.Fatal(err)
	}

	f := CmdImportFlags{
		Ip:    6,
		Size:  32,
		Merge: "none",
		In:    inputFile,
		Out:   outputFile,
		Csv:   true,
	}

	err := CmdImport(f, []string{}, func() {})
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	verifyMMDBContent(t, outputFile, []struct {
		ip       string
		expected map[string]interface{}
	}{
		{
			ip: "167.153.128.1",
			expected: map[string]interface{}{
				"network": "167.153.128.0/17",
				"country": "US",
			},
		},
		{
			ip: "204.138.232.1",
			expected: map[string]interface{}{
				"network": "204.138.232.0/24",
				"country": "CA",
			},
		},
	})
}

func TestCmdImport_TSVWithContent(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.tsv")
	outputFile := filepath.Join(tempDir, "output.mmdb")

	tsvData := "network\tcountry\tcity\n167.153.128.0/17\tUS\tNew York\n204.138.232.0/24\tCA\tToronto\n"
	if err := os.WriteFile(inputFile, []byte(tsvData), 0644); err != nil {
		t.Fatal(err)
	}

	f := CmdImportFlags{
		Ip:    6,
		Size:  32,
		Merge: "none",
		In:    inputFile,
		Out:   outputFile,
		Tsv:   true,
	}

	err := CmdImport(f, []string{}, func() {})
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	verifyMMDBContent(t, outputFile, []struct {
		ip       string
		expected map[string]interface{}
	}{
		{
			ip: "167.153.128.1",
			expected: map[string]interface{}{
				"network": "167.153.128.0/17",
				"country": "US",
				"city":    "New York",
			},
		},
		{
			ip: "204.138.232.1",
			expected: map[string]interface{}{
				"network": "204.138.232.0/24",
				"country": "CA",
				"city":    "Toronto",
			},
		},
	})
}

func TestCmdImport_JSONWithContent(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.json")
	outputFile := filepath.Join(tempDir, "output.mmdb")

	jsonData := `{"range": "167.153.128.0/17", "country": "US", "asn": 12345, "active": true}
{"range": "204.138.232.0/24", "country": "CA", "asn": 67890, "active": false}`
	if err := os.WriteFile(inputFile, []byte(jsonData), 0644); err != nil {
		t.Fatal(err)
	}

	f := CmdImportFlags{
		Ip:    6,
		Size:  32,
		Merge: "none",
		In:    inputFile,
		Out:   outputFile,
		Json:  true,
	}

	err := CmdImport(f, []string{}, func() {})
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	verifyMMDBContent(t, outputFile, []struct {
		ip       string
		expected map[string]interface{}
	}{
		{
			ip: "167.153.128.1",
			expected: map[string]interface{}{
				"network": "167.153.128.0/17",
				"country": "US",
				"asn":     float64(12345), // JSON numbers become float64
				"active":  true,
			},
		},
		{
			ip: "204.138.232.1",
			expected: map[string]interface{}{
				"network": "204.138.232.0/24",
				"country": "CA",
				"asn":     float64(67890),
				"active":  false,
			},
		},
	})
}

func TestCmdImport_JSONArray(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.json")
	outputFile := filepath.Join(tempDir, "output.mmdb")

	jsonData := `[
  {"range": "167.153.128.0/17", "country": "US"},
  {"start_ip": "204.138.232.0", "end_ip": "204.138.232.255", "country": "CA"}
]
`
	if err := os.WriteFile(inputFile, []byte(jsonData), 0644); err != nil {
		t.Fatal(err)
	}

	f := CmdImportFlags{
		Ip:    6,
		Size:  32,
		Merge: "none",
		In:    inputFile,
		Out:   outputFile,
		Json:  true,
	}

	if err := CmdImport(f, []string{}, func() {}); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	verifyMMDBContent(t, outputFile, []struct {
		ip       string
		expected map[string]interface{}
	}{
		{
			ip: "167.153.128.1",
			expected: map[string]interface{}{
				"network": "167.153.128.0/17",
				"country": "US",
			},
		},
		{
			ip: "204.138.232.1",
			expected: map[string]interface{}{
				"network": "204.138.232.0-204.138.232.255",
				"country": "CA",
			},
		},
	})
}

func TestCmdImport_MalformedJSON(t *testing.T) {
	// An empty want takes any error: where an unclosed array is noticed
	// depends on the Go release's decoder.
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{"a string", `"just a string"`, "record 1 is not a JSON object"},
		{"an array element", `[{"range": "1.0.0.0/24"}, 7]`, "record 2 is not a JSON object"},
		{"no end_ip", `{"start_ip": "1.0.0.0"}`, "record 1 has a start_ip but no end_ip"},
		{"no network", `{"country": "US"}`, "record 1 has no range, or start_ip and end_ip"},
		{"invalid", `{"range": }`, "record 1: invalid character"},
		{"an unclosed array", `[{"range": "1.0.0.0/24"}`, ""},
		{"more after the array", `[{"range": "1.0.0.0/24"}] {"range": "2.0.0.0/24"}`,
			"the input goes on after the array of records"},
		{"an empty array", `[]`, "nothing to import"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			inputFile := filepath.Join(tempDir, "input.json")
			outputFile := filepath.Join(tempDir, "output.mmdb")
			if err := os.WriteFile(inputFile, []byte(tc.input), 0644); err != nil {
				t.Fatal(err)
			}

			f := CmdImportFlags{
				Ip:    6,
				Size:  32,
				Merge: "none",
				In:    inputFile,
				Out:   outputFile,
				Json:  true,
			}

			err := CmdImport(f, []string{}, func() {})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want an error containing %q", err, tc.want)
			}
			if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
				t.Errorf("a failed import left %v", outputFile)
			}
		})
	}
}

func TestCmdImport_IPRangeWithContent(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.csv")
	outputFile := filepath.Join(tempDir, "output.mmdb")

	csvData := "start_ip,end_ip,country\n167.153.128.0,167.153.255.255,US\n204.138.232.0,204.138.232.255,CA\n"
	if err := os.WriteFile(inputFile, []byte(csvData), 0644); err != nil {
		t.Fatal(err)
	}

	f := CmdImportFlags{
		Ip:    6,
		Size:  32,
		Merge: "none",
		In:    inputFile,
		Out:   outputFile,
		Csv:   true,
	}

	err := CmdImport(f, []string{}, func() {})
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	verifyMMDBContent(t, outputFile, []struct {
		ip       string
		expected map[string]interface{}
	}{
		{
			ip: "167.153.200.1",
			expected: map[string]interface{}{
				"network": "167.153.128.0-167.153.255.255",
				"country": "US",
			},
		},
		{
			ip: "204.138.232.100",
			expected: map[string]interface{}{
				"network": "204.138.232.0-204.138.232.255",
				"country": "CA",
			},
		},
	})
}

func TestParseCSVHeaders(t *testing.T) {
	tests := []struct {
		name          string
		parts         []string
		expectRange   bool
		expectJoinKey bool
		expectStart   int
	}{
		{
			name:          "simple header",
			parts:         []string{"network", "country"},
			expectRange:   false,
			expectJoinKey: false,
			expectStart:   1,
		},
		{
			name:          "range header",
			parts:         []string{"start_ip", "end_ip", "country"},
			expectRange:   true,
			expectJoinKey: false,
			expectStart:   2,
		},
		{
			name:          "range with join key",
			parts:         []string{"start_ip", "end_ip", "join_key", "country"},
			expectRange:   true,
			expectJoinKey: true,
			expectStart:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CmdImportFlags{FieldsFromHdr: true}
			dataColStart := 1

			ParseCSVHeaders(tt.parts, f, &dataColStart)

			if f.RangeMultiCol != tt.expectRange {
				t.Errorf("expected RangeMultiCol to be %v, got %v", tt.expectRange, f.RangeMultiCol)
			}
			if f.JoinKeyCol != tt.expectJoinKey {
				t.Errorf("expected JoinKeyCol to be %v, got %v", tt.expectJoinKey, f.JoinKeyCol)
			}
			if dataColStart != tt.expectStart {
				t.Errorf("expected dataColStart to be %d, got %d", tt.expectStart, dataColStart)
			}
		})
	}
}

func TestConvertToMMDBType(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string", "test", "string"},
		{"float64", float64(123.45), "float64"},
		{"int32", int32(123), "int32"},
		{"bool", true, "bool"},
		{"nil", nil, "string"}, // nil converts to empty string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertToMMDBType(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %s", err.Error())
			}
			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestCmdImport_NoNetwork(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.csv")
	outputFile := filepath.Join(tempDir, "output.mmdb")

	csvData := "network,country,city\n167.153.128.0/17,US,New York\n204.138.232.0/24,CA,Toronto\n"
	if err := os.WriteFile(inputFile, []byte(csvData), 0644); err != nil {
		t.Fatal(err)
	}

	f := CmdImportFlags{
		Ip:        6,
		Size:      32,
		Merge:     "none",
		In:        inputFile,
		Out:       outputFile,
		Csv:       true,
		NoNetwork: true,
	}

	err := CmdImport(f, []string{}, func() {})
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	// The "network" field should NOT be present
	verifyMMDBContent(t, outputFile, []struct {
		ip       string
		expected map[string]interface{}
	}{
		{
			ip: "167.153.128.1",
			expected: map[string]interface{}{
				"country": "US",
				"city":    "New York",
			},
		},
		{
			ip: "204.138.232.1",
			expected: map[string]interface{}{
				"country": "CA",
				"city":    "Toronto",
			},
		},
	})
}
