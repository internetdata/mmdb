package lib

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataTypeSizes(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		want TypeSizes
	}{
		{"map of strings", "e1" + "446e616d65" + "4568656c6c6f", TypeSizes{Utf8StringSize: 9, MapKeyValueCount: 1}},
		{"pointers of every size", "2000" + "280000" + "30000000" + "3800000000", TypeSizes{PointerSize: 10}},
		{"double", "68" + zeros(8), TypeSizes{DoubleSize: 8}},
		{"float", "0408" + zeros(4), TypeSizes{FloatSize: 4}},
		{"bytes", "83" + zeros(3), TypeSizes{BytesSize: 3}},
		{"uint16", "a2" + zeros(2), TypeSizes{Unsigned16bitIntSize: 2}},
		{"uint32", "c3" + zeros(3), TypeSizes{Unsigned32bitIntSize: 3}},
		{"int32", "0401" + zeros(4), TypeSizes{Signed32bitIntSize: 4}},
		{"uint64", "0802" + zeros(8), TypeSizes{Unsigned64bitIntSize: 8}},
		{"uint128", "1003" + zeros(16), TypeSizes{Unsigned128bitIntSize: 16}},
		{"array of booleans", "0204" + "0107" + "0007", TypeSizes{ArrayLength: 2}},
		{"one-byte size", "5d05" + zeros(34), TypeSizes{Utf8StringSize: 34}},
		{"two-byte size", "5e0001" + zeros(286), TypeSizes{Utf8StringSize: 286}},
		{"three-byte size", "5f000001" + zeros(65822), TypeSizes{Utf8StringSize: 65822}},
		{"empty", "", TypeSizes{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dataTypeSizes(unhex(t, tc.data))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDataTypeSizes_Malformed(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{"size cut short", "5e00", "runs past the end"},
		{"payload cut short", "456865", "runs past the end"},
		{"pointer cut short", "4568656c6c6f" + "2800", "offset 6 runs past the end"},
		{"no extended type", "00", "runs past the end"},
		{"extended type below 8", "0000", "invalid extended type"},
		{"unknown type", "0009", "unknown data type 16"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dataTypeSizes(unhex(t, tc.data))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got error %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestCmdMetadata_DataTypes(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")
	out := filepath.Join(dir, "out.mmdb")
	if err := os.WriteFile(in, []byte("network,name\n1.0.0.0/24,hello\n2.0.0.0/24,hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := CmdImportFlags{Ip: 6, Size: 24, Merge: "none", In: in, Out: out, Csv: true}
	if err := CmdImport(f, []string{}, func() {}); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() error {
		return CmdMetadata(CmdMetadataFlags{Format: "json", DataTypes: true}, []string{out}, func() {})
	})
	var md struct {
		DataSectionSize int        `json:"data_section_size"`
		TypeSizes       *TypeSizes `json:"data_type_sizes"`
	}
	if err := json.Unmarshal([]byte(stdout), &md); err != nil {
		t.Fatalf("%v in %q", err, stdout)
	}
	// Two maps of name and network, the second pointing at the first's name, hello and network.
	want := TypeSizes{PointerSize: 3, Utf8StringSize: 36, MapKeyValueCount: 4}
	if md.DataSectionSize != 49 || md.TypeSizes == nil || *md.TypeSizes != want {
		t.Errorf("got a %d-byte data section holding %+v, want 49 bytes holding %+v",
			md.DataSectionSize, md.TypeSizes, want)
	}
}

func zeros(n int) string {
	return strings.Repeat("00", n)
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
