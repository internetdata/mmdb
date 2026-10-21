// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	iputil "github.com/mslmio/libgo-iputil"

	"github.com/maxmind/mmdbwriter/v2"
	"github.com/maxmind/mmdbwriter/v2/inserter"
	"github.com/maxmind/mmdbwriter/v2/mmdbtype"
	"github.com/spf13/pflag"
)

// CmdImportFlags are flags expected by CmdImport.
type CmdImportFlags struct {
	Help                bool
	In                  string
	Out                 string
	Backup              string
	Csv                 bool
	Tsv                 bool
	Json                bool
	Fields              []string
	FieldsFromHdr       bool
	RangeMultiCol       bool
	JoinKeyCol          bool
	NoFields            bool
	NoNetwork           bool
	Ip                  int
	Size                int // 24, 28 or 32, or 0 for the smallest that fits
	Merge               string
	IgnoreEmptyVals     bool
	DisallowReserved    bool
	Alias6to4           bool
	DisableMetadataPtrs bool
	DryRun              bool
}

var CmdImportFlagsDefaults = CmdImportFlags{
	Help:                false,
	In:                  "",
	Out:                 "",
	Backup:              "",
	Csv:                 false,
	Tsv:                 false,
	Json:                false,
	Fields:              nil,
	FieldsFromHdr:       false,
	RangeMultiCol:       false,
	JoinKeyCol:          false,
	NoFields:            false,
	NoNetwork:           false,
	Ip:                  6,
	Size:                0,
	Merge:               "none",
	IgnoreEmptyVals:     false,
	DisallowReserved:    false,
	Alias6to4:           false,
	DisableMetadataPtrs: true,
	DryRun:              false,
}

// Init initializes the common flags available to CmdImport with sensible
// defaults.
//
// pflag.Parse() must be called to actually use the final flag values.
func (f *CmdImportFlags) Init() {
	_h := "see description in --help"
	pflag.BoolVarP(
		&f.Help,
		"help", "h", CmdImportFlagsDefaults.Help,
		"show help.",
	)
	pflag.StringVarP(
		&f.In,
		"in", "i", CmdImportFlagsDefaults.In,
		_h,
	)
	pflag.StringVarP(
		&f.Out,
		"out", "o", CmdImportFlagsDefaults.Out,
		_h,
	)
	pflag.StringVar(
		&f.Backup,
		"backup", CmdImportFlagsDefaults.Backup,
		_h,
	)
	pflag.BoolVarP(
		&f.Csv,
		"csv", "c", CmdImportFlagsDefaults.Csv,
		_h,
	)
	pflag.BoolVarP(
		&f.Tsv,
		"tsv", "t", CmdImportFlagsDefaults.Tsv,
		_h,
	)
	pflag.BoolVarP(
		&f.Json,
		"json", "j", CmdImportFlagsDefaults.Json,
		_h,
	)
	pflag.StringSliceVarP(
		&f.Fields,
		"fields", "f", CmdImportFlagsDefaults.Fields,
		_h,
	)
	pflag.BoolVar(
		&f.FieldsFromHdr,
		"fields-from-header", CmdImportFlagsDefaults.FieldsFromHdr,
		_h,
	)
	pflag.BoolVar(
		&f.RangeMultiCol,
		"range-multicol", CmdImportFlagsDefaults.RangeMultiCol,
		_h,
	)
	pflag.BoolVar(
		&f.JoinKeyCol,
		"joinkey-col", CmdImportFlagsDefaults.JoinKeyCol,
		_h,
	)
	pflag.BoolVar(
		&f.NoFields,
		"no-fields", CmdImportFlagsDefaults.NoFields,
		_h,
	)
	pflag.BoolVar(
		&f.NoNetwork,
		"no-network", CmdImportFlagsDefaults.NoNetwork,
		_h,
	)
	pflag.IntVar(
		&f.Ip,
		"ip", CmdImportFlagsDefaults.Ip,
		_h,
	)
	f.Size = CmdImportFlagsDefaults.Size
	pflag.VarP(
		(*recordSizeFlag)(&f.Size),
		"size", "s",
		_h,
	)
	pflag.StringVarP(
		&f.Merge,
		"merge", "m", CmdImportFlagsDefaults.Merge,
		_h,
	)
	pflag.BoolVar(
		&f.IgnoreEmptyVals,
		"ignore-empty-values", CmdImportFlagsDefaults.IgnoreEmptyVals,
		_h,
	)
	pflag.BoolVar(
		&f.DisallowReserved,
		"disallow-reserved", CmdImportFlagsDefaults.DisallowReserved,
		_h,
	)
	pflag.BoolVar(
		&f.Alias6to4,
		"alias-6to4", CmdImportFlagsDefaults.Alias6to4,
		_h,
	)
	pflag.BoolVar(
		&f.DisableMetadataPtrs,
		"disable-metadata-pointers", CmdImportFlagsDefaults.DisableMetadataPtrs,
		_h,
	)
	pflag.BoolVar(
		&f.DryRun,
		"dry-run", CmdImportFlagsDefaults.DryRun,
		_h,
	)
}

func CmdImport(f CmdImportFlags, args []string, printHelp func()) error {
	// help?
	if f.Help || (pflag.NArg() == 1 && pflag.NFlag() == 0) {
		printHelp()
		return nil
	}

	// optional input as 1st and output as 2nd argument.
	if len(args) >= 2 {
		f.In = args[0]
		f.Out = args[1]
	}

	// validate IP version.
	if f.Ip != 4 && f.Ip != 6 {
		return errors.New("ip version must be \"4\" or \"6\"")
	}

	// validate record size.
	if f.Size != 0 && f.Size != 24 && f.Size != 28 && f.Size != 32 {
		return errors.New("record size must be auto, 24, 28 or 32")
	}

	// validate merge strategy.
	var mergeStrategy inserter.PureFunc
	if f.Merge == "none" {
		// nil is the writer's direct replace path, with no inserter overhead.
		mergeStrategy = nil
	} else if f.Merge == "toplevel" {
		mergeStrategy = inserter.TopLevelMerge
	} else if f.Merge == "recurse" {
		mergeStrategy = inserter.DeepMerge
	} else {
		return errors.New("merge strategy must be \"none\", \"toplevel\" or \"recurse\"")
	}

	// figure out file type.
	var delim rune
	if !f.Csv && !f.Tsv && !f.Json {
		if strings.HasSuffix(f.In, ".csv") {
			delim = ','
		} else if strings.HasSuffix(f.In, ".tsv") {
			delim = '\t'
		} else if strings.HasSuffix(f.In, ".json") {
			delim = '-'
		} else {
			return errors.New("input file type unknown")
		}
	} else {
		if f.Csv && f.Tsv || f.Csv && f.Json || f.Tsv && f.Json {
			return errors.New("multiple input file types specified")
		} else if f.Csv {
			delim = ','
		} else if f.Tsv {
			delim = '\t'
		} else {
			delim = '-'
		}
	}

	// figure out fields.
	fieldSrcCnt := 0
	if len(f.Fields) > 0 {
		fieldSrcCnt += 1
	}
	if f.FieldsFromHdr {
		fieldSrcCnt += 1
	}
	if f.NoFields {
		fieldSrcCnt += 1
	}
	if fieldSrcCnt > 1 {
		return errors.New("conflicting field sources specified")
	}
	if f.NoFields {
		f.Fields = []string{}
		f.NoNetwork = false
	} else if !f.FieldsFromHdr && len(f.Fields) == 0 {
		f.FieldsFromHdr = true
	}

	if f.JoinKeyCol {
		f.RangeMultiCol = true
	}

	// figure out backup name.
	backup := ""
	if f.Backup != "" || pflag.CommandLine.Changed("backup") {
		if f.Out == "" {
			return errors.New("--backup needs --out")
		}
		var err error
		backup, err = backupName(f.Out, f.Backup)
		if err != nil {
			return err
		}
	}

	// prepare output file; a dry run writes none.
	var out *outFile
	if f.Out != "" && !f.DryRun {
		if backup != "" {
			if err := checkBackup(f.Out, backup, f.In); err != nil {
				return err
			}
		}
		var err error
		out, err = createOut(f.Out, backup)
		if err != nil {
			return err
		}
		defer out.abort()
	}

	// init tree, or what a dry run counts in its place.
	dbdesc := "internetdata " + filepath.Base(f.Out)
	opts := mmdbwriter.Options{
		DatabaseType: dbdesc,
		Description: map[string]string{
			"en": dbdesc,
		},
		Languages:               []string{"en"},
		DisableIPv4Aliasing:     !f.Alias6to4,
		IncludeReservedNetworks: !f.DisallowReserved,
		IPVersion:               f.Ip,
		RecordSize:              f.Size,
		DisableMetadataPointers: f.DisableMetadataPtrs,
		Inserter:                mergeStrategy,
	}
	var tree *mmdbwriter.Tree
	var dry *dryRun
	var target importTarget
	var err error
	if f.DryRun {
		dry, err = newDryRun(opts)
		target = dry
	} else {
		tree, err = mmdbwriter.New(opts)
		target = tree
	}
	if err != nil {
		return fmt.Errorf("could not create tree: %w", err)
	}

	// prepare input file.
	var inFile *os.File
	if f.In == "" || f.In == "-" {
		inFile = os.Stdin
	} else {
		var err error
		inFile, err = os.Open(f.In)
		if err != nil {
			return fmt.Errorf("invalid input file %v: %w", f.In, err)
		}
		defer inFile.Close()
	}

	inFileBuffered := bufio.NewReaderSize(inFile, 65536)

	entrycnt := 0
	if delim == ',' || delim == '\t' {
		var rdr reader
		if delim == ',' {
			csvrdr := csv.NewReader(inFileBuffered)
			csvrdr.Comma = delim
			csvrdr.LazyQuotes = true

			rdr = csvrdr
		} else {
			tsvrdr := NewTsvReader(inFileBuffered)

			rdr = tsvrdr
		}

		// read from input, scanning & parsing each line according to delim,
		// then insert that into the tree.
		dataColStart := 1
		hdrSeen := false
		for {
			parts, err := rdr.Read()
			if err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("input scanning failed: %w", err)
			}

			// on header line?
			if !hdrSeen {
				hdrSeen = true

				ParseCSVHeaders(parts, &f, &dataColStart)

				// Now that f.Fields may have been resolved, the preprocessing step can be run
				err = Preprocess(f, target)
				if err != nil {
					return err
				}

				// should we skip this first line now?
				if f.FieldsFromHdr {
					continue
				}
			}

			err = AppendCSVRecord(f, dataColStart, delim, parts, target)
			if err != nil {
				return err
			}

			entrycnt += 1
		}
	} else if delim == '-' {
		// The records are objects one after another, or the elements of one
		// top-level array; either way they are decoded one at a time.
		inArray := false
		for {
			b, err := inFileBuffered.Peek(1)
			if err != nil || strings.IndexByte(" \t\r\n", b[0]) < 0 {
				inArray = err == nil && b[0] == '['
				break
			}
			inFileBuffered.Discard(1)
		}
		dataStream := json.NewDecoder(inFileBuffered)
		if inArray {
			if _, err := dataStream.Token(); err != nil {
				return fmt.Errorf("couldn't read the array of records: %w", err)
			}
		}

		// For JSON input, f.Fields may have been specified using the --fields flag, so preprocessing can be run
		err = Preprocess(f, target)
		if err != nil {
			return err
		}

		fieldsResolved := false
		for n := 1; ; n++ {
			if inArray && !dataStream.More() {
				break
			}
			var row interface{}
			err := dataStream.Decode(&row)
			if err == io.EOF && !inArray {
				break
			}
			if err != nil {
				return fmt.Errorf("record %d: %w", n, err)
			}
			mResult, ok := row.(map[string]interface{})
			if !ok {
				return fmt.Errorf("record %d is not a JSON object", n)
			}

			if !fieldsResolved {
				fieldsResolved = true
				ParseJSONKeys(mResult, &f)
			}

			// convert 2 IPs into IP range?
			var networkStr string
			if val, ok := mResult["start_ip"].(string); ok {
				endIP, ok := mResult["end_ip"].(string)
				if !ok {
					return fmt.Errorf("record %d has a start_ip but no end_ip", n)
				}
				networkStr = val + "-" + endIP
				delete(mResult, "start_ip")
				delete(mResult, "end_ip")
				if _, ok := mResult["join_key"].(string); ok {
					delete(mResult, "join_key")
				}
			} else if val, ok := mResult["range"].(string); ok {
				networkStr = val
				delete(mResult, "range")
			} else {
				return fmt.Errorf("record %d has no range, or start_ip and end_ip", n)
			}

			// add network part to single-IP network if it's missing.
			isNetworkRange := strings.Contains(networkStr, "-")
			if !isNetworkRange && !strings.Contains(networkStr, "/") {
				if f.Ip == 6 && strings.Contains(networkStr, ":") {
					networkStr += "/128"
				} else {
					networkStr += "/32"
				}
			}
			subMap := mmdbtype.Map{}
			if !f.NoNetwork {
				subMap["network"] = mmdbtype.String(networkStr)
			}

			// prep record.
			errProcessData := ProcessJsonData(mResult, f, &subMap)
			if errProcessData != nil {
				return fmt.Errorf("failed to map to mmdb.type err: %w", errProcessData)
			}

			// range insertion or cidr insertion?
			if isNetworkRange {
				networkStrParts := strings.Split(networkStr, "-")
				startIp, _ := netip.ParseAddr(networkStrParts[0])
				endIp, _ := netip.ParseAddr(networkStrParts[1])
				if err := target.InsertRange(startIp, endIp, subMap); err != nil {
					fmt.Fprintf(
						os.Stderr, "warn: couldn't insert '%v'\n",
						mResult,
					)
				}
			} else {
				network, err := netip.ParsePrefix(networkStr)
				if err != nil {
					return fmt.Errorf(
						"couldn't parse cidr \"%v\": %w",
						networkStr, err,
					)
				}
				if err := target.Insert(network, subMap); err != nil {
					fmt.Fprintf(
						os.Stderr, "warn: couldn't insert '%v'\n",
						mResult,
					)
				}
			}

			entrycnt += 1
		}
		if inArray {
			if _, err := dataStream.Token(); err != nil {
				return fmt.Errorf("the array of records is not closed: %w", err)
			}
			if _, err := dataStream.Token(); err != io.EOF {
				return errors.New("the input goes on after the array of records")
			}
		}
	}

	if entrycnt == 0 {
		return errors.New("nothing to import")
	}

	if f.DryRun {
		report, err := dry.report(entrycnt)
		if err != nil {
			return fmt.Errorf("dry run failed: %w", err)
		}
		report.print(os.Stdout, f.Out, f.Size)
		return nil
	}

	// write out mmdb file.
	fmt.Fprintf(os.Stderr, "writing to %s (%v entries)\n", f.Out, entrycnt)
	if out == nil {
		_, err = tree.WriteTo(os.Stdout)
	} else if _, err = tree.WriteTo(out.File); err == nil {
		err = out.commit()
	}
	if err != nil {
		return fmt.Errorf("writing out to tree failed: %w", err)
	}

	return nil
}

// recordSizeFlag is --size: auto, stored as 0, or a number of bits, which
// CmdImport validates.
type recordSizeFlag int

func (s *recordSizeFlag) String() string {
	if *s == 0 {
		return "auto"
	}
	return strconv.Itoa(int(*s))
}

func (s *recordSizeFlag) Set(v string) error {
	if v == "auto" {
		*s = 0
		return nil
	}
	n, err := strconv.ParseInt(v, 0, 0)
	if err != nil {
		return errors.New("record size must be auto, 24, 28 or 32")
	}
	*s = recordSizeFlag(n)
	return nil
}

func (s *recordSizeFlag) Type() string {
	return "string"
}

func Preprocess(f CmdImportFlags, tree importTarget) error {
	// insert empty values for all fields in 0.0.0.0/0 if requested.
	if f.IgnoreEmptyVals {
		network := netip.MustParsePrefix("0.0.0.0/0")
		record := mmdbtype.Map{}
		for _, field := range f.Fields {
			record[mmdbtype.String(field)] = mmdbtype.String("")
		}
		if err := tree.Insert(network, record); err != nil {
			return errors.New(
				"couldn't insert empty values to 0.0.0.0/0",
			)
		}
	}

	return nil
}

func ParseCSVHeaders(parts []string, f *CmdImportFlags, dataColStart *int) {
	// check if the header has a multi-column range.
	if len(parts) > 1 && parts[0] == "start_ip" && parts[1] == "end_ip" {
		f.RangeMultiCol = true

		// maybe we also have a join key?
		if len(parts) > 2 && parts[2] == "join_key" {
			f.JoinKeyCol = true
		}
	}

	if f.RangeMultiCol {
		if f.JoinKeyCol {
			*dataColStart = 3
		} else {
			*dataColStart = 2
		}
	}

	// need to get fields from hdr?
	if f.FieldsFromHdr {
		// skip all non-data columns.
		f.Fields = parts[*dataColStart:]
	}
}

func ParseJSONKeys(result map[string]interface{}, f *CmdImportFlags) {
	if _, hasStartIp := result["start_ip"].(string); hasStartIp {
		if _, hasEndIp := result["end_ip"].(string); hasEndIp {
			f.RangeMultiCol = true

			if _, hasJoinKey := result["join_key"].(string); hasJoinKey {
				f.JoinKeyCol = true
			}
		}
	}

	// determine fields
	// NOTE: even though there are no headers for JSON, we reuse that variable to signal the need to extract fields
	if f.FieldsFromHdr {
		for key := range result {
			switch key {
			case "start_ip", "end_ip", "join_key", "range":
				continue
			default:
				f.Fields = append(f.Fields, key)
			}
		}
	}
}

func AppendCSVRecord(f CmdImportFlags, dataColStart int, delim rune, parts []string, tree importTarget) error {
	if startIP, err := iputil.AddrFromDecimal(parts[0], false); err == nil {
		parts[0] = startIP.String()
	}

	networkStr := parts[0]

	// convert 2 IPs into IP range?
	if f.RangeMultiCol {
		if endIP, err := iputil.AddrFromDecimal(parts[1], false); err == nil {
			parts[1] = endIP.String()
		}

		networkStr = parts[0] + "-" + parts[1]
	}

	// add network part to single-IP network if it's missing.
	isNetworkRange := strings.Contains(networkStr, "-")
	if !isNetworkRange && !strings.Contains(networkStr, "/") {
		if f.Ip == 6 && strings.Contains(networkStr, ":") {
			networkStr += "/128"
		} else {
			networkStr += "/32"
		}
	}

	// prep record.
	record := mmdbtype.Map{}
	if !f.NoNetwork {
		record["network"] = mmdbtype.String(networkStr)
	}
	for i, field := range f.Fields {
		record[mmdbtype.String(field)] = mmdbtype.String(parts[i+dataColStart])
	}

	// range insertion or cidr insertion?
	if isNetworkRange {
		networkStrParts := strings.Split(networkStr, "-")
		startIp, _ := netip.ParseAddr(networkStrParts[0])
		endIp, _ := netip.ParseAddr(networkStrParts[1])
		if err := tree.InsertRange(startIp, endIp, record); err != nil {
			fmt.Fprintf(
				os.Stderr, "warn: couldn't insert line '%v'\n",
				strings.Join(parts, string(delim)),
			)
		}
	} else {
		network, err := netip.ParsePrefix(networkStr)
		if err != nil {
			return fmt.Errorf(
				"couldn't parse cidr \"%v\": %w",
				networkStr, err,
			)
		}
		if err := tree.Insert(network, record); err != nil {
			fmt.Fprintf(
				os.Stderr, "warn: couldn't insert line '%v'\n",
				strings.Join(parts, string(delim)),
			)
		}
	}

	return nil
}

func ProcessJsonData(
	data map[string]interface{},
	f CmdImportFlags,
	subMap *mmdbtype.Map,
) error {
	// Insert each key-value pair into the map
	for _, field := range f.Fields {
		value, ok := data[field]
		if !ok {
			continue
		}

		mmdbValue, err := ConvertToMMDBType(value)
		if err != nil {
			return fmt.Errorf("failed to convert value to MMDB type: %v", err)
		}
		(*subMap)[mmdbtype.String(field)] = mmdbValue
	}

	return nil
}

func ConvertToMMDBType(value interface{}) (mmdbtype.DataType, error) {
	switch v := value.(type) {
	case nil:
		return mmdbtype.String(""), nil
	case string:
		return mmdbtype.String(v), nil
	case float64:
		return mmdbtype.Float64(v), nil
	case float32:
		return mmdbtype.Float32(v), nil
	case int32:
		return mmdbtype.Int32(v), nil
	case uint16:
		return mmdbtype.Uint16(v), nil
	case uint32:
		return mmdbtype.Uint32(v), nil
	case uint64:
		return mmdbtype.Uint64(v), nil
	case bool:
		return mmdbtype.Bool(v), nil
	case map[string]interface{}:
		subMap := mmdbtype.Map{}
		for key, val := range v {
			mmdbValue, err := ConvertToMMDBType(val)
			if err != nil {
				return nil, fmt.Errorf("failed to convert value to MMDB type: %v", err)
			}
			subMap[mmdbtype.String(key)] = mmdbValue
		}
		return subMap, nil
	case []interface{}:
		subSlice := mmdbtype.Slice{}
		for _, val := range v {
			mmdbValue, err := ConvertToMMDBType(val)
			if err != nil {
				return nil, fmt.Errorf("failed to convert value to MMDB type: %v", err)
			}
			subSlice = append(subSlice, mmdbValue)
		}
		return subSlice, nil
	default:
		outJson, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return mmdbtype.String(outJson), nil
	}
}
