// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"

	iputil "github.com/mslmio/libgo-iputil"

	"github.com/fatih/color"
	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/spf13/pflag"
)

var predictReadFmts = []string{
	"json",
	"json-compact",
	"json-pretty",
	"tsv",
	"csv",
}

// readCacheCap bounds the records read keeps; at the cap it starts over.
var readCacheCap = 1 << 16

// CmdReadFlags are flags expected by CmdRead.
type CmdReadFlags struct {
	Help    bool
	NoColor bool
	Format  string
}

// Init initializes the common flags available to CmdRead with sensible
// defaults.
//
// pflag.Parse() must be called to actually use the final flag values.
func (f *CmdReadFlags) Init() {
	_h := "see description in --help"
	pflag.BoolVarP(
		&f.Help,
		"help", "h", false,
		"show help.",
	)
	pflag.BoolVar(
		&f.NoColor,
		"nocolor", false,
		_h,
	)
	pflag.StringVarP(
		&f.Format,
		"format", "f", "json",
		_h,
	)
}

func CmdRead(f CmdReadFlags, args []string, printHelp func()) error {
	if f.NoColor {
		color.NoColor = true
	}

	// help? with no argument there is no mmdb file to read, whatever the flags.
	if f.Help || len(args) == 0 {
		printHelp()
		return nil
	}

	// validate format.
	if f.Format == "json" {
		f.Format = "json-compact"
	}
	validFormat := false
	for _, format := range predictReadFmts {
		if f.Format == format {
			validFormat = true
			break
		}
	}
	if !validFormat {
		return fmt.Errorf("format must be one of %v", predictReadFmts)
	}

	// last arg must be mmdb file; open it.
	mmdbFileArg := args[len(args)-1]
	db, err := maxminddb.Open(mmdbFileArg)
	if err != nil {
		return fmt.Errorf("couldn't open mmdb file %v: %w", mmdbFileArg, err)
	}
	defer db.Close()

	requiresHdr := f.Format == "csv" || f.Format == "tsv"
	hdrWritten := false
	var wr writer
	if f.Format == "csv" {
		csvwr := csv.NewWriter(os.Stdout)
		wr = csvwr
	} else if f.Format == "tsv" {
		tsvwr := NewTsvWriter(os.Stdout)
		wr = tsvwr
	}

	cache := make(map[uintptr]readRecord)
	var row []string
	var line []byte
	var pretty bytes.Buffer

	// Streamed rather than collected into a slice first: the arguments accepted
	// here include CIDRs, and `mmdb read 0.0.0.0/0 db.mmdb` would otherwise try
	// to hold 4.3 billion addresses before looking up the first one.
	err = iputil.WalkAddrs(args[:len(args)-1], iputil.ListOpts, func(addr netip.Addr) error {
		var rec readRecord
		result := db.Lookup(addr.Unmap())
		if result.Found() {
			rec = cachedReadRecord(cache, result, requiresHdr)
		}
		if !rec.found {
			if !requiresHdr {
				fmt.Fprintf(os.Stderr,
					"err: couldn't get data for %s\n",
					addr.String(),
				)
			}
			return nil
		}

		if !hdrWritten {
			hdrWritten = true

			if requiresHdr {
				hdr := append([]string{"ip"}, rec.keys...)
				if err := wr.Write(hdr); err != nil {
					return fmt.Errorf(
						"failed to write header %v: %w",
						hdr, err,
					)
				}
			}
		}

		if requiresHdr {
			row = append(append(row[:0], addr.String()), rec.vals...)
			if err := wr.Write(row); err != nil {
				return fmt.Errorf("failed to write line %v: %w", row, err)
			}
			return nil
		}

		if rec.jsonOK {
			line = append(line[:0], rec.json.line[:rec.json.cut]...)
			line = addr.AppendTo(line)
			line = append(line, rec.json.line[rec.json.cut:]...)
		}
		if rec.jsonOK && f.Format == "json-pretty" {
			pretty.Reset()
			rec.jsonOK = json.Indent(&pretty, line, "", "  ") == nil
			line = append(line[:0], pretty.Bytes()...)
		}
		if !rec.jsonOK {
			fmt.Fprintf(os.Stderr,
				"err: couldn't print data for %s\n",
				addr.String(),
			)
			return nil
		}
		// One write per line, so a pipeline reading it gets each line as it is looked up.
		line = append(line, '\n')
		os.Stdout.Write(line)
		return nil
	})
	if err != nil {
		return err
	}

	if wr != nil {
		wr.Flush()
		if err := wr.Error(); err != nil {
			return fmt.Errorf("writer had failure: %w", err)
		}
	}

	return nil
}

// readRecord is what read prints for one record, kept by its offset in the data
// section, since a long list of addresses shares far fewer records.
type readRecord struct {
	found  bool
	keys   []string
	vals   []string
	json   jsonRecord
	jsonOK bool
}

func cachedReadRecord(cache map[uintptr]readRecord, result maxminddb.Result, flat bool) readRecord {
	offset := result.Offset()
	if rec, ok := cache[offset]; ok {
		return rec
	}
	rec := newReadRecord(result, flat)
	if len(cache) >= readCacheCap {
		clear(cache)
	}
	cache[offset] = rec
	return rec
}

// newReadRecord renders a record as CSV and TSV columns when flat, and as JSON
// cut where "ip" sorts otherwise: the address looked up replaces any "ip" of its own.
func newReadRecord(result maxminddb.Result, flat bool) readRecord {
	record := make(map[string]any)
	if err := result.Decode(&record); err != nil || len(record) == 0 {
		return readRecord{}
	}
	if flat {
		keys, vals := sortedMapKeysAndVals(mapInterfaceToStr(record))
		return readRecord{found: true, keys: keys, vals: vals}
	}
	encoded, err := encodeJSONRecord(record, "ip")
	return readRecord{found: true, json: encoded, jsonOK: err == nil}
}
