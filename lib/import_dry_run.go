package lib

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"maps"
	"math/bits"
	"net/netip"
	"slices"

	"github.com/maxmind/mmdbwriter/v2"
	"github.com/maxmind/mmdbwriter/v2/mmdbtype"
	"github.com/oschwald/maxminddb-golang/v2"
	"go4.org/netipx"
)

// importTarget is what an import inserts into: the tree, or a dry run.
type importTarget interface {
	Insert(network netip.Prefix, value mmdbtype.DataType) error
	InsertRange(start, end netip.Addr, value mmdbtype.DataType) error
}

// A build's peak memory per counted node, distinct record and network, and
// at the least: fitted to four builds of 0.9M to 120M networks.
const (
	dryRunBytesPerNode    = 42
	dryRunBytesPerRecord  = 400
	dryRunBytesPerNetwork = 30
	dryRunBaseBytes       = 16 << 20
)

// metadataMarker opens an mmdb file's metadata, which follows its data section.
var metadataMarker = []byte("\xab\xcd\xefMaxMind.com")

// dryRun stands in for the tree when import is given --dry-run. It keeps each
// network in 24 bytes and each distinct record once, and from them reports
// what the build would need, without building it.
type dryRun struct {
	opts    mmdbwriter.Options
	nets    []dryRunNet
	ids     map[[16]byte]uint32
	records *mmdbwriter.Tree
}

// dryRunNet is a network as the writer places it: an IPv4 network in an IPv6
// tree sits under ::/96, 96 bits deeper.
type dryRunNet struct {
	hi, lo uint64
	id     uint32
	bits   uint8
}

// dryRunReport is what a dry run found.
type dryRunReport struct {
	Rows      int
	Networks  int
	Records   int
	Nodes     uint64
	DataBytes int64
}

func newDryRun(opts mmdbwriter.Options) (*dryRun, error) {
	// Each distinct record is written once, at a network of its own, so that
	// the writer itself says how large the data section is.
	records, err := mmdbwriter.New(mmdbwriter.Options{
		IncludeReservedNetworks: true,
		DisableIPv4Aliasing:     true,
		DisableMetadataPointers: opts.DisableMetadataPointers,
		RecordSize:              32,
	})
	if err != nil {
		return nil, err
	}
	return &dryRun{opts: opts, ids: map[[16]byte]uint32{}, records: records}, nil
}

func (d *dryRun) Insert(network netip.Prefix, value mmdbtype.DataType) error {
	id, err := d.recordID(value)
	if err != nil {
		return err
	}
	return d.add(network, id)
}

func (d *dryRun) InsertRange(start, end netip.Addr, value mmdbtype.DataType) error {
	start, end = start.Unmap(), end.Unmap()
	r := netipx.IPRangeFrom(start, end)
	if !r.IsValid() {
		return errors.New("start & end IPs did not give valid range")
	}
	if d.opts.IPVersion == 4 && !start.Is4() {
		return errors.New("IPv6 ranges cannot be inserted into an IPv4 tree")
	}
	id, err := d.recordID(value)
	if err != nil {
		return err
	}
	for _, network := range r.Prefixes() {
		if err := d.add(network, id); err != nil {
			return err
		}
	}
	return nil
}

// add keeps a network the way the tree would place it, refusing what the
// tree refuses.
func (d *dryRun) add(network netip.Prefix, id uint32) error {
	if !network.IsValid() {
		return errors.New("prefix is invalid")
	}
	addr, size := network.Addr(), network.Bits()
	if addr.Is4In6() {
		if size < 96 {
			return errors.New("IPv4-mapped prefixes shorter than /96 cannot be inserted")
		}
		addr, size = addr.Unmap(), size-96
	}
	network = netip.PrefixFrom(addr, size).Masked()
	addr = network.Addr()

	n := dryRunNet{id: id, bits: uint8(size)}
	if addr.Is4() {
		v4 := addr.As4()
		if d.opts.IPVersion == 4 {
			n.hi = uint64(binary.BigEndian.Uint32(v4[:])) << 32
		} else {
			n.lo = uint64(binary.BigEndian.Uint32(v4[:]))
			n.bits += 96
		}
	} else {
		if d.opts.IPVersion == 4 {
			return errors.New("IPv6 prefixes cannot be inserted into an IPv4 tree")
		}
		v6 := addr.As16()
		n.hi = binary.BigEndian.Uint64(v6[:8])
		n.lo = binary.BigEndian.Uint64(v6[8:])
	}
	d.nets = append(d.nets, n)
	return nil
}

// recordID numbers each distinct record, adding a new one to the records
// tree. Record i sits at 8000::/1 plus i in the next 32 bits, so the tree's
// node count follows from how many there are (recordsTreeNodes).
func (d *dryRun) recordID(value mmdbtype.DataType) (uint32, error) {
	h := fnv.New128a()
	hashValue(h, value)
	var key [16]byte
	h.Sum(key[:0])
	if id, ok := d.ids[key]; ok {
		return id, nil
	}
	if len(d.ids) == 1<<32-1 {
		return 0, errors.New("more distinct records than any mmdb file can hold")
	}
	id := uint32(len(d.ids))
	var a [16]byte
	binary.BigEndian.PutUint64(a[:8], 1<<63|uint64(id)<<31)
	err := d.records.Insert(netip.PrefixFrom(netip.AddrFrom16(a), 33), value)
	if err != nil {
		return 0, err
	}
	d.ids[key] = id
	return id, nil
}

// hashValue writes a value to h so that two values hash alike only when the
// writer would store them as one record.
func hashValue(h hash.Hash, value mmdbtype.DataType) {
	var n [8]byte
	writeLen := func(l int) {
		binary.BigEndian.PutUint64(n[:], uint64(l))
		h.Write(n[:])
	}
	switch v := value.(type) {
	case mmdbtype.String:
		h.Write([]byte{'s'})
		writeLen(len(v))
		h.Write([]byte(v))
	case mmdbtype.Map:
		h.Write([]byte{'m'})
		writeLen(len(v))
		for _, k := range slices.Sorted(maps.Keys(v)) {
			writeLen(len(k))
			h.Write([]byte(k))
			hashValue(h, v[k])
		}
	case mmdbtype.Slice:
		h.Write([]byte{'l'})
		writeLen(len(v))
		for _, e := range v {
			hashValue(h, e)
		}
	default:
		s := fmt.Sprintf("%T:%v", v, v)
		h.Write([]byte{'o'})
		writeLen(len(s))
		h.Write([]byte(s))
	}
}

// report counts what the build would need. The node count is an upper bound:
// it merges two neighbors that carry the same record, as the writer does, but
// leaves out the identical subtrees the writer then stores once.
func (d *dryRun) report(rows int) (dryRunReport, error) {
	r := dryRunReport{Rows: rows, Networks: len(d.nets), Records: len(d.ids)}

	slices.SortFunc(d.nets, compareDryRunNets)
	nets := mergeSiblings(d.nets)
	r.Nodes = countNodes(nets)

	// The aliases and reserved networks the writer adds before any row, whose
	// root the rows share.
	base, err := emptyTreeNodes(d.opts)
	if err != nil {
		return r, err
	}
	if base > 0 && r.Nodes > 0 {
		base--
	}
	r.Nodes += base

	r.DataBytes, err = d.dataBytes()
	return r, err
}

func compareDryRunNets(a, b dryRunNet) int {
	if a.hi != b.hi {
		return cmpUint64(a.hi, b.hi)
	}
	if a.lo != b.lo {
		return cmpUint64(a.lo, b.lo)
	}
	return int(a.bits) - int(b.bits)
}

func cmpUint64(a, b uint64) int {
	if a < b {
		return -1
	}
	return 1
}

// mergeSiblings replaces two sibling networks carrying the same record with
// their parent, as the writer does, all the way up. Sorted input keeps a left
// sibling just before its right one; a network in between means the two
// overlap others, and they are left as they are.
func mergeSiblings(nets []dryRunNet) []dryRunNet {
	out := nets[:0]
	for _, n := range nets {
		for len(out) > 0 && n.bits > 0 {
			top := out[len(out)-1]
			if top.bits != n.bits || top.id != n.id || commonBits(top, n) != int(n.bits)-1 {
				break
			}
			out = out[:len(out)-1]
			n = parentNet(n)
		}
		out = append(out, n)
	}
	return out
}

// countNodes counts the nodes on the paths to sorted networks. A network of n
// bits has its record in the node at depth n-1, so its path is n nodes deep,
// and it shares with the network before it every node down to the depth where
// the two part.
func countNodes(nets []dryRunNet) uint64 {
	var nodes uint64
	for i, n := range nets {
		if i == 0 {
			nodes += uint64(n.bits)
			continue
		}
		p := nets[i-1]
		shared := min(commonBits(p, n), int(p.bits)-1, int(n.bits)-1)
		nodes += uint64(int(n.bits) - 1 - shared)
	}
	return nodes
}

func commonBits(a, b dryRunNet) int {
	if x := a.hi ^ b.hi; x != 0 {
		return bits.LeadingZeros64(x)
	}
	return 64 + bits.LeadingZeros64(a.lo^b.lo)
}

func parentNet(n dryRunNet) dryRunNet {
	n.bits--
	if n.bits < 64 {
		n.hi &^= 1 << (63 - n.bits)
	} else {
		n.lo &^= 1 << (127 - n.bits)
	}
	return n
}

// emptyTreeNodes is the node count of a tree built with opts and holding no
// rows: the paths to its IPv4 aliases and reserved networks.
func emptyTreeNodes(opts mmdbwriter.Options) (uint64, error) {
	opts.Inserter = nil
	tree, err := mmdbwriter.New(opts)
	if err != nil {
		return 0, err
	}
	var buf bytes.Buffer
	if _, err := tree.WriteTo(&buf); err != nil {
		return 0, err
	}
	db, err := maxminddb.OpenBytes(buf.Bytes())
	if err != nil {
		return 0, err
	}
	return uint64(db.Metadata.NodeCount), nil
}

// dataBytes writes the records tree nowhere, and reads the size of its data
// section off the stream: the tree, 16 zero bytes, the data, then metadata.
func (d *dryRun) dataBytes() (int64, error) {
	if len(d.ids) == 0 {
		return 0, nil
	}
	var w tailWriter
	if _, err := d.records.WriteTo(&w); err != nil {
		return 0, err
	}
	i := bytes.LastIndex(w.tail, metadataMarker)
	if i < 0 {
		return 0, errors.New("the records' metadata wasn't found")
	}
	marker := w.n - int64(len(w.tail)) + int64(i)
	return marker - int64(recordsTreeNodes(len(d.ids)))*8 - 16, nil
}

// recordsTreeNodes is the node count of a records tree holding n records: the
// root, then a complete tree of 32 levels whose leaves are 0 to n-1.
func recordsTreeNodes(n int) uint64 {
	nodes := uint64(1)
	for depth := range 32 {
		nodes += uint64(n-1)>>(32-depth) + 1
	}
	return nodes
}

// tailWriter counts what it is given and keeps the last of it, where the
// metadata is.
type tailWriter struct {
	n    int64
	tail []byte
}

const tailKeep = 128 << 10

func (w *tailWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	w.tail = append(w.tail, p...)
	if len(w.tail) > 2*tailKeep {
		w.tail = append(w.tail[:0], w.tail[len(w.tail)-tailKeep:]...)
	}
	return len(p), nil
}

// print writes the report for a build of out at the given record size.
func (r dryRunReport) print(w io.Writer, out string, recordSize int) {
	if out == "" {
		fmt.Fprintln(w, "dry run; nothing was written")
	} else {
		fmt.Fprintf(w, "dry run for %s; nothing was written\n", out)
	}
	fmt.Fprintf(
		w, "  input:   %s rows, %s networks, %s distinct records\n",
		humanCount(uint64(r.Rows)), humanCount(uint64(r.Networks)), humanCount(uint64(r.Records)),
	)
	fmt.Fprintf(
		w, "  tree:    at most %s nodes and %s of records\n",
		humanCount(r.Nodes), humanBytes(r.DataBytes, 1000, "B", "kB", "MB", "GB", "TB"),
	)
	fmt.Fprintf(w, "  fits:    %s\n", r.fits(recordSize))
	mem := int64(r.Nodes)*dryRunBytesPerNode + int64(r.Records)*dryRunBytesPerRecord +
		int64(r.Networks)*dryRunBytesPerNetwork + dryRunBaseBytes
	fmt.Fprintf(
		w, "  memory:  about %s at the peak, while writing\n",
		humanBytes(mem, 1024, "B", "KiB", "MiB", "GiB", "TiB"),
	)
}

// fits says which record sizes can address the tree and its records, and how
// full the chosen one would be.
func (r dryRunReport) fits(recordSize int) string {
	// A record points at a node or into the data section, past the node
	// count and the 16-byte separator.
	used := float64(r.Nodes) + 16 + float64(r.DataBytes)
	var sizes []string
	for _, size := range []int{24, 28, 32} {
		if used <= float64(uint64(1)<<size) {
			sizes = append(sizes, fmt.Sprint(size))
		}
	}
	full := 100 * used / float64(uint64(1)<<recordSize)
	var in string
	switch len(sizes) {
	case 0:
		return fmt.Sprintf("may not: up to %s of the 32-bit space", percent(100*used/(1<<32)))
	case 1:
		in = "in 32-bit records only"
	case 2:
		in = "in 28- and 32-bit records"
	default:
		in = "in 24-, 28- and 32-bit records"
	}
	if full > 100 {
		return fmt.Sprintf("%s, not --size %d", in, recordSize)
	}
	if len(sizes) == 1 {
		return fmt.Sprintf("%s, at most %s full", in, percent(full))
	}
	return fmt.Sprintf("%s; %d-bit at most %s full", in, recordSize, percent(full))
}

// percent rounds a share up to two decimals below 1%, and to a whole one above.
func percent(p float64) string {
	if p < 1 {
		return fmt.Sprintf("%.2f%%", max(p, 0.01))
	}
	return fmt.Sprintf("%.0f%%", p)
}

// humanCount writes a count to three or four significant digits: 909.6K,
// 119.5M, 3.48B.
func humanCount(n uint64) string {
	switch {
	case n < 1000:
		return fmt.Sprint(n)
	case n < 1e6:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	case n < 1e9:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	default:
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	}
}

// humanBytes writes a size in the largest unit that keeps it at least 1.
func humanBytes(n int64, base float64, units ...string) string {
	v := float64(n)
	i := 0
	for v >= base && i < len(units)-1 {
		v /= base
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[0])
	}
	if v < 10 {
		return fmt.Sprintf("%.2f %s", v, units[i])
	}
	return fmt.Sprintf("%.0f %s", v, units[i])
}
