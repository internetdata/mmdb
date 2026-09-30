package lib

import (
	"encoding/binary"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/oschwald/maxminddb-golang/v2"
)

func TestCmdCompress_ShrinksAnUnsharedTree(t *testing.T) {
	for _, tc := range []struct {
		size    int
		aliased bool
	}{{24, true}, {28, true}, {32, true}, {32, false}} {
		t.Run(strconv.Itoa(tc.size)+"/aliased="+strconv.FormatBool(tc.aliased), func(t *testing.T) {
			dir := t.TempDir()
			in := filepath.Join(dir, "in.mmdb")
			out := filepath.Join(dir, "out.mmdb")
			writeUnsharedMMDB(t, in, tc.size, tc.aliased)

			if err := CmdCompress(CmdCompressFlags{}, []string{in, out}, func() {}); err != nil {
				t.Fatalf("compress failed: %s", err)
			}

			before := openMMDB(t, in)
			after := openMMDB(t, out)
			if after.Metadata.NodeCount >= before.Metadata.NodeCount {
				t.Errorf("node count went from %d to %d", before.Metadata.NodeCount, after.Metadata.NodeCount)
			}
			if after.Metadata.RecordSize != uint(tc.size) || after.Metadata.BuildEpoch != before.Metadata.BuildEpoch ||
				after.Metadata.DatabaseType != before.Metadata.DatabaseType ||
				after.Metadata.Description["en"] != before.Metadata.Description["en"] {
				t.Errorf("metadata changed: %+v -> %+v", before.Metadata, after.Metadata)
			}
			for _, ip := range []string{
				"1.0.0.1", "1.0.1.1", "2.0.0.1", "2.0.1.1", "3.0.0.1", "2600::1", "2601::1",
				"::ffff:1.0.1.1", "2002:100:101::", "2001:0:200:1::", "2003::",
			} {
				want, got := lookup(t, before, ip), lookup(t, after, ip)
				if want != got {
					t.Errorf("%s answered %q before and %q after", ip, want, got)
				}
			}
			st, err := openSearchTree(out, after.Metadata)
			if err != nil {
				t.Fatal(err)
			}
			defer st.file.Close()
			ipv4Root, err := st.ipv4Root()
			if err != nil {
				t.Fatal(err)
			}
			if aliased, err := st.aliasesIPv4(ipv4Root); err != nil || aliased != tc.aliased {
				t.Errorf("output aliases IPv4: %v (%v), want %v", aliased, err, tc.aliased)
			}
		})
	}
}

func TestCmdCompress_RefusesACompressedFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.mmdb")
	compressed := filepath.Join(dir, "compressed.mmdb")
	again := filepath.Join(dir, "again.mmdb")
	writeUnsharedMMDB(t, in, 32, true)
	if err := CmdCompress(CmdCompressFlags{}, []string{in, compressed}, func() {}); err != nil {
		t.Fatalf("compress failed: %s", err)
	}

	err := CmdCompress(CmdCompressFlags{}, []string{compressed, again}, func() {})
	if err == nil || !strings.Contains(err.Error(), "already compressed") {
		t.Fatalf("expected an already compressed error, got %v", err)
	}
	if _, err := os.Stat(again); !os.IsNotExist(err) {
		t.Errorf("expected no output file, got %v", err)
	}
}

func TestCmdCompress_RefusesAnImportedFile(t *testing.T) {
	dir := t.TempDir()
	csv := filepath.Join(dir, "in.csv")
	in := filepath.Join(dir, "in.mmdb")
	out := filepath.Join(dir, "out.mmdb")
	rows := "network,asn\n1.0.0.0/24,1\n1.0.1.0/24,2\n2.0.0.0/24,1\n2.0.1.0/24,2\n"
	if err := os.WriteFile(csv, []byte(rows), 0644); err != nil {
		t.Fatal(err)
	}
	err := CmdImport(CmdImportFlags{
		Ip: 6, Size: 32, Merge: "none", In: csv, Out: in, Csv: true, NoNetwork: true, FieldsFromHdr: true,
	}, []string{}, func() {})
	if err != nil {
		t.Fatalf("import failed: %s", err)
	}

	err = CmdCompress(CmdCompressFlags{}, []string{in, out}, func() {})
	if err == nil || !strings.Contains(err.Error(), "already compressed") {
		t.Fatalf("expected an already compressed error, got %v", err)
	}
}

func TestCmdCompress_BadArguments(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.mmdb")
	writeUnsharedMMDB(t, in, 32, true)

	for name, args := range map[string][]string{
		"one file":        {in},
		"same file":       {in, in},
		"missing input":   {filepath.Join(dir, "missing.mmdb"), filepath.Join(dir, "out.mmdb")},
		"not an mmdb":     {filepath.Join(dir), filepath.Join(dir, "out.mmdb")},
		"three arguments": {in, filepath.Join(dir, "a.mmdb"), filepath.Join(dir, "b.mmdb")},
	} {
		if err := CmdCompress(CmdCompressFlags{}, args, func() {}); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "out.mmdb")); !os.IsNotExist(err) {
		t.Errorf("expected no output file, got %v", err)
	}
}

func openMMDB(t *testing.T, path string) *maxminddb.Reader {
	t.Helper()
	db, err := maxminddb.Open(path)
	if err != nil {
		t.Fatalf("couldn't open %s: %s", path, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func lookup(t *testing.T, db *maxminddb.Reader, ip string) string {
	t.Helper()
	var v string
	if err := db.Lookup(netip.MustParseAddr(ip)).Decode(&v); err != nil {
		t.Fatalf("lookup of %s failed: %s", ip, err)
	}
	return v
}

// writeUnsharedMMDB writes an IPv6 tree as a writer without subtree sharing
// does: 1.0.0.0/23 and 2.0.0.0/23 hold identical subtrees stored twice, and
// aliased adds the ::ffff:0:0/96, 2001::/32 and 2002::/16 aliases.
func writeUnsharedMMDB(t *testing.T, path string, recordSize int, aliased bool) {
	t.Helper()
	type node struct {
		kids  [2]*node
		value [2]string
		alias [2]bool
	}
	root := &node{}
	walk := func(prefix string) (*node, int) {
		p := netip.MustParsePrefix(prefix)
		addr := p.Addr().As16()
		n := root
		for bit := 0; bit < p.Bits()-1; bit++ {
			b := addr[bit/8] >> (7 - bit%8) & 1
			if n.kids[b] == nil {
				n.kids[b] = &node{}
			}
			n = n.kids[b]
		}
		bit := p.Bits() - 1
		return n, int(addr[bit/8] >> (7 - bit%8) & 1)
	}
	for prefix, value := range map[string]string{
		"::100:0/120": "x", "::100:100/120": "y", "::200:0/120": "x", "::200:100/120": "y", "2600::/16": "v6",
	} {
		n, b := walk(prefix)
		n.value[b] = value
	}
	for _, alias := range []string{"::ffff:0:0/96", "2001::/32", "2002::/16"} {
		if aliased {
			n, b := walk(alias)
			n.alias[b] = true
		}
	}

	var nodes []*node
	numbers := map[*node]uint32{}
	var number func(n *node)
	number = func(n *node) {
		numbers[n] = uint32(len(nodes))
		nodes = append(nodes, n)
		for _, kid := range n.kids {
			if kid != nil {
				number(kid)
			}
		}
	}
	number(root)
	ipv4Root, _ := walk("::/97")
	nodeCount := uint32(len(nodes))

	var data []byte
	offsets := map[string]uint32{}
	var tree []byte
	for _, n := range nodes {
		var records [2]uint32
		for b := range 2 {
			switch {
			case n.kids[b] != nil:
				records[b] = numbers[n.kids[b]]
			case n.alias[b]:
				records[b] = numbers[ipv4Root]
			case n.value[b] != "":
				if _, ok := offsets[n.value[b]]; !ok {
					offsets[n.value[b]] = uint32(len(data))
					data = append(data, mmdbString(n.value[b])...)
				}
				records[b] = nodeCount + 16 + offsets[n.value[b]]
			default:
				records[b] = nodeCount
			}
		}
		l, r := records[0], records[1]
		switch recordSize {
		case 24:
			tree = append(tree, byte(l>>16), byte(l>>8), byte(l), byte(r>>16), byte(r>>8), byte(r))
		case 28:
			tree = append(tree, byte(l>>16), byte(l>>8), byte(l), byte(l>>20&0xF0|r>>24&0x0F),
				byte(r>>16), byte(r>>8), byte(r))
		default:
			tree = binary.BigEndian.AppendUint32(tree, l)
			tree = binary.BigEndian.AppendUint32(tree, r)
		}
	}

	file := append(tree, make([]byte, 16)...)
	file = append(file, data...)
	file = append(file, MetadataStartMarker...)
	file = append(file, 0xE9)
	file = append(file, mmdbString("binary_format_major_version")...)
	file = append(file, 0xA1, 2)
	file = append(file, mmdbString("binary_format_minor_version")...)
	file = append(file, 0xA0)
	file = append(file, mmdbString("build_epoch")...)
	file = append(file, 0x04, 0x02, 0x65, 0x00, 0x00, 0x00)
	file = append(file, mmdbString("database_type")...)
	file = append(file, mmdbString("unshared")...)
	file = append(file, mmdbString("description")...)
	file = append(file, 0xE1)
	file = append(file, mmdbString("en")...)
	file = append(file, mmdbString("no shared subtrees")...)
	file = append(file, mmdbString("ip_version")...)
	file = append(file, 0xA1, 6)
	file = append(file, mmdbString("languages")...)
	file = append(file, 0x01, 0x04)
	file = append(file, mmdbString("en")...)
	file = append(file, mmdbString("node_count")...)
	file = append(file, 0xC4)
	file = binary.BigEndian.AppendUint32(file, nodeCount)
	file = append(file, mmdbString("record_size")...)
	file = append(file, 0xA1, byte(recordSize))
	if err := os.WriteFile(path, file, 0644); err != nil {
		t.Fatal(err)
	}
}

// mmdbString encodes a UTF-8 string of under 29 bytes in the data section's
// format.
func mmdbString(s string) []byte {
	return append([]byte{0x40 | byte(len(s))}, s...)
}
