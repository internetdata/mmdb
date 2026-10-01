package lib

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"

	"github.com/maxmind/mmdbwriter/v2"
	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/spf13/pflag"
)

// CmdCompressFlags are flags expected by CmdCompress.
type CmdCompressFlags struct {
	Help bool
}

// Init initializes the common flags available to CmdCompress with sensible
// defaults.
//
// pflag.Parse() must be called to actually use the final flag values.
func (f *CmdCompressFlags) Init() {
	pflag.BoolVarP(
		&f.Help,
		"help", "h", false,
		"show help.",
	)
}

// CmdCompress rewrites an mmdb file so that each distinct subtree of its
// search tree is stored once, leaving every lookup's answer as it was.
func CmdCompress(f CmdCompressFlags, args []string, printHelp func()) error {
	// help?
	if f.Help || (pflag.NArg() == 1 && pflag.NFlag() == 0) {
		printHelp()
		return nil
	}

	if len(args) != 2 {
		return errors.New("input and output mmdb files required as arguments")
	}
	in, out := args[0], args[1]

	inStat, err := os.Stat(in)
	if err != nil {
		return fmt.Errorf("couldn't open mmdb file: %w", err)
	}
	if outStat, err := os.Stat(out); err == nil && os.SameFile(inStat, outStat) {
		return errors.New("output must be a different file than the input")
	}

	db, err := maxminddb.Open(in)
	if err != nil {
		return fmt.Errorf("couldn't open mmdb file: %w", err)
	}
	md := db.Metadata
	db.Close()

	st, err := openSearchTree(in, md)
	if err != nil {
		return err
	}
	defer st.file.Close()

	ipv4Root, err := st.ipv4Root()
	if err != nil {
		return err
	}
	shared, err := st.sharesSubtrees(ipv4Root)
	if err != nil {
		return err
	}
	if shared {
		return fmt.Errorf("%v is already compressed, so nothing was written", in)
	}
	aliased, err := st.aliasesIPv4(ipv4Root)
	if err != nil {
		return err
	}

	// Every network of the input is kept, reserved ones included, and its
	// IPv4 aliases only if it had them.
	tree, err := mmdbwriter.Load(in, mmdbwriter.Options{
		BuildEpoch:              int64(md.BuildEpoch),
		DisableIPv4Aliasing:     !aliased,
		IncludeReservedNetworks: true,
		DisableMetadataPointers: true,
	})
	if err != nil {
		return fmt.Errorf("couldn't load %v: %w", in, err)
	}

	outFile, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("could not create %v: %w", out, err)
	}
	written, err := tree.WriteTo(outFile)
	if err == nil {
		err = outFile.Close()
	} else {
		outFile.Close()
	}
	if err != nil {
		os.Remove(out)
		return fmt.Errorf("writing %v failed: %w", out, err)
	}

	fmt.Fprintf(
		os.Stderr, "wrote %v: %v -> %v bytes (%+.1f%%)\n",
		out, inStat.Size(), written,
		100*(float64(written)/float64(inStat.Size())-1),
	)
	return nil
}

// searchTree reads the nodes of an mmdb file's search tree, which the reader
// library does not expose.
type searchTree struct {
	file       *os.File
	nodeCount  uint32
	nodeSize   int
	recordSize uint
	ipVersion  uint
}

func openSearchTree(path string, md maxminddb.Metadata) (*searchTree, error) {
	if md.RecordSize != 24 && md.RecordSize != 28 && md.RecordSize != 32 {
		return nil, fmt.Errorf("unsupported record size %v", md.RecordSize)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't open mmdb file: %w", err)
	}
	return &searchTree{
		file:       file,
		nodeCount:  uint32(md.NodeCount),
		nodeSize:   int(md.RecordSize) / 4,
		recordSize: md.RecordSize,
		ipVersion:  md.IPVersion,
	}, nil
}

// sharesSubtrees reports whether a node is the child of two records, which
// only a writer that stores identical subtrees once produces. ipv4Root is
// exempt, since any writer may alias it from ::ffff:0:0/96, 2001::/32 and 2002::/16.
func (st *searchTree) sharesSubtrees(ipv4Root uint32) (bool, error) {
	seen := make([]uint64, (int64(st.nodeCount)+63)/64)
	nodes := bufio.NewReaderSize(
		io.NewSectionReader(st.file, 0, int64(st.nodeCount)*int64(st.nodeSize)), 1<<20,
	)
	node := make([]byte, st.nodeSize)
	for range st.nodeCount {
		if _, err := io.ReadFull(nodes, node); err != nil {
			return false, fmt.Errorf("couldn't read the search tree: %w", err)
		}
		for _, child := range st.records(node) {
			if child >= st.nodeCount || child == ipv4Root {
				continue
			}
			if seen[child/64]&(1<<(child%64)) != 0 {
				return true, nil
			}
			seen[child/64] |= 1 << (child % 64)
		}
	}
	return false, nil
}

// ipv4Root is the node at ::/96, or nodeCount when there is none.
func (st *searchTree) ipv4Root() (uint32, error) {
	if st.ipVersion != 6 {
		return st.nodeCount, nil
	}
	r, err := st.record(netip.MustParsePrefix("::/96"))
	return min(r, st.nodeCount), err
}

func (st *searchTree) aliasesIPv4(ipv4Root uint32) (bool, error) {
	if ipv4Root >= st.nodeCount {
		return false, nil
	}
	for _, alias := range []string{"::ffff:0:0/96", "2001::/32", "2002::/16"} {
		r, err := st.record(netip.MustParsePrefix(alias))
		if err != nil || r != ipv4Root {
			return false, err
		}
	}
	return true, nil
}

// record follows prefix from the root and returns the record it ends on, or
// the first one on its way that is not a node.
func (st *searchTree) record(prefix netip.Prefix) (uint32, error) {
	addr := prefix.Addr().As16()
	node := make([]byte, st.nodeSize)
	r := uint32(0)
	for bit := 0; bit < prefix.Bits() && r < st.nodeCount; bit++ {
		_, err := st.file.ReadAt(node, int64(r)*int64(st.nodeSize))
		if err != nil {
			return 0, fmt.Errorf("couldn't read the search tree: %w", err)
		}
		r = st.records(node)[addr[bit/8]>>(7-bit%8)&1]
	}
	return r, nil
}

func (st *searchTree) records(node []byte) [2]uint32 {
	switch st.recordSize {
	case 24:
		return [2]uint32{
			uint32(node[0])<<16 | uint32(node[1])<<8 | uint32(node[2]),
			uint32(node[3])<<16 | uint32(node[4])<<8 | uint32(node[5]),
		}
	case 28:
		return [2]uint32{
			uint32(node[3]&0xF0)<<20 | uint32(node[0])<<16 | uint32(node[1])<<8 | uint32(node[2]),
			uint32(node[3]&0x0F)<<24 | uint32(node[4])<<16 | uint32(node[5])<<8 | uint32(node[6]),
		}
	default:
		return [2]uint32{binary.BigEndian.Uint32(node[:4]), binary.BigEndian.Uint32(node[4:])}
	}
}
