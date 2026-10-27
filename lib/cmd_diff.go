// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"reflect"
	"slices"

	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/spf13/pflag"
)

// CmdDiffFlags are flags expected by CmdDiff.
type CmdDiffFlags struct {
	Help    bool
	Subnets bool
	Records bool
}

// Init initializes the common flags available to CmdDiff with sensible
// defaults.
//
// pflag.Parse() must be called to actually use the final flag values.
func (f *CmdDiffFlags) Init() {
	_h := "see description in --help"
	pflag.BoolVarP(
		&f.Help,
		"help", "h", false,
		"show help.",
	)
	pflag.BoolVarP(
		&f.Subnets,
		"subnets", "s", false,
		_h,
	)
	pflag.BoolVarP(
		&f.Records,
		"records", "r", false,
		_h,
	)
}

type cmdDiffRecord struct {
	oldr interface{}
	newr interface{}
}

// diffCacheCap bounds each of diffRecords' caches; at the cap one starts over.
var diffCacheCap = 1 << 16

// diffRecords decodes each file's records once, by offset, and compares each
// pair of offsets once: two files' offsets never compare directly.
type diffRecords struct {
	newRecs map[uintptr]interface{}
	oldRecs map[uintptr]interface{}
	equal   map[[2]uintptr]bool
}

func newDiffRecords() *diffRecords {
	return &diffRecords{
		newRecs: make(map[uintptr]interface{}),
		oldRecs: make(map[uintptr]interface{}),
		equal:   make(map[[2]uintptr]bool),
	}
}

// doDiff walks newDb's networks and looks each up in oldDb. Records are only
// compared when records is not nil.
func doDiff(
	newDb *maxminddb.Reader,
	newDbStr string,
	oldDb *maxminddb.Reader,
	oldDbStr string,
	records *diffRecords,
) (map[netip.Prefix]netip.Prefix, map[netip.Prefix]cmdDiffRecord, error) {
	modifiedSubnets := map[netip.Prefix]netip.Prefix{}
	modifiedRecords := map[netip.Prefix]cmdDiffRecord{}
	for result := range newDb.Networks() {
		if err := result.Err(); err != nil {
			return nil, nil, fmt.Errorf(
				"failed to get record for subnet from %v: %w",
				newDbStr, err,
			)
		}
		subnetA := result.Prefix()

		lookupResult := oldDb.Lookup(subnetA.Addr())
		if err := lookupResult.Err(); err != nil {
			return nil, nil, fmt.Errorf(
				"failed to get record for IP %v from %v: %w",
				subnetA.Addr(), oldDbStr, err,
			)
		}
		subnetB := lookupResult.Prefix()

		// unequal subnets?
		if subnetA != subnetB {
			modifiedSubnets[subnetA] = subnetB
			continue
		}
		if records == nil {
			continue
		}

		// different data for same subnet?
		pair := [2]uintptr{result.Offset(), lookupResult.Offset()}
		equal, known := records.equal[pair]
		if known && equal {
			continue
		}
		recordA, err := decodeCached(records.newRecs, result)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"failed to get record for subnet from %v: %w",
				newDbStr, err,
			)
		}
		recordB, err := decodeCached(records.oldRecs, lookupResult)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"failed to get record for IP %v from %v: %w",
				subnetA.Addr(), oldDbStr, err,
			)
		}
		if !known {
			equal = reflect.DeepEqual(recordA, recordB)
			if len(records.equal) >= diffCacheCap {
				clear(records.equal)
			}
			records.equal[pair] = equal
		}
		if !equal {
			modifiedRecords[subnetA] = cmdDiffRecord{
				oldr: recordB,
				newr: recordA,
			}
		}
	}

	return modifiedSubnets, modifiedRecords, nil
}

// decodeCached decodes a result's record once per offset. A result that found
// no record decodes to nil.
func decodeCached(cache map[uintptr]interface{}, result maxminddb.Result) (interface{}, error) {
	if !result.Found() {
		return nil, nil
	}
	if record, ok := cache[result.Offset()]; ok {
		return record, nil
	}
	var record interface{}
	if err := result.Decode(&record); err != nil {
		return nil, err
	}
	if len(cache) >= diffCacheCap {
		clear(cache)
	}
	cache[result.Offset()] = record
	return record, nil
}

func CmdDiff(f CmdDiffFlags, args []string, printHelp func()) error {
	if f.Help || (pflag.NArg() == 1 && pflag.NFlag() == 0) {
		printHelp()
		return nil
	}

	// validate input files.
	if len(args) != 2 {
		return errors.New("two input mmdb file required as arguments")
	}

	// open old db.
	oldMmdb := args[0]
	oldDb, err := maxminddb.Open(oldMmdb)
	if err != nil {
		return fmt.Errorf("couldnt open %v: %w", oldMmdb, err)
	}
	defer oldDb.Close()

	// open new db.
	newMmdb := args[1]
	newDb, err := maxminddb.Open(newMmdb)
	if err != nil {
		return fmt.Errorf("couldnt open %v: %w", newMmdb, err)
	}
	defer newDb.Close()

	// confirm that they're of the same IP version.
	if newDb.Metadata.IPVersion != oldDb.Metadata.IPVersion {
		return fmt.Errorf(
			"IP versions differ between files: %v=%v and %v=%v",
			newMmdb, newDb.Metadata.IPVersion,
			oldMmdb, oldDb.Metadata.IPVersion,
		)
	}

	// collect set difference data.
	ambSn, ambRec, err := doDiff(newDb, newMmdb, oldDb, oldMmdb, newDiffRecords())
	if err != nil {
		return err
	}
	bmaSn, _, err := doDiff(oldDb, oldMmdb, newDb, newMmdb, nil)
	if err != nil {
		return err
	}

	// print.
	if f.Subnets {
		if len(ambSn) > 0 || len(bmaSn) > 0 {
			fmt.Println("** SUBNETS **")
			changes := make([][2]netip.Prefix, 0, len(ambSn)+len(bmaSn))
			for newSn, oldSn := range ambSn {
				changes = append(changes, [2]netip.Prefix{oldSn, newSn})
			}
			for oldSn, newSn := range bmaSn {
				changes = append(changes, [2]netip.Prefix{oldSn, newSn})
			}
			slices.SortFunc(changes, func(a, b [2]netip.Prefix) int {
				return cmp.Or(a[0].Compare(b[0]), a[1].Compare(b[1]))
			})
			for _, sn := range changes {
				fmt.Printf("%v -> %v\n", sn[0], sn[1])
			}
		}
		fmt.Println(len(ambSn)+len(bmaSn), "subnet(s) modified.")
	}
	if f.Records {
		if f.Subnets {
			fmt.Println()
		}

		if len(ambRec) > 0 {
			fmt.Println("** RECORDS **")
			for _, sn := range slices.SortedFunc(maps.Keys(ambRec), netip.Prefix.Compare) {
				fmt.Println(sn)
				fmt.Printf("	-%v\n", ambRec[sn].oldr)
				fmt.Printf("	+%v\n", ambRec[sn].newr)
			}
		}
		fmt.Println(len(ambRec), "record(s) modified.")
	}
	if !f.Subnets && !f.Records {
		fmt.Println(len(ambSn)+len(bmaSn), "subnet(s) modified.")
		fmt.Println(len(ambRec), "record(s) modified.")
	}

	return nil
}
