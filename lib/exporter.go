// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

import (
	"fmt"
	"net/netip"

	"github.com/oschwald/maxminddb-golang/v2"
)

// exporter defines the interface for exporting MMDB records.
type exporter interface {
	WriteRecord(result maxminddb.Result) error
	Flush() error
}

// exportNetworks iterates over all networks in the database and writes them using the exporter.
func exportNetworks(db *maxminddb.Reader, exp exporter) error {
	for result := range db.Networks() {
		if err := result.Err(); err != nil {
			return fmt.Errorf("failed networks traversal: %w", err)
		}
		if err := exp.WriteRecord(result); err != nil {
			return err
		}
	}
	return exp.Flush()
}

// rangeStr renders a network for the range column. A single address is written
// bare, as in the CSVs these databases are built from, unless cidrOnly is set.
func rangeStr(prefix netip.Prefix, cidrOnly bool) string {
	if !cidrOnly && prefix.IsSingleIP() {
		return prefix.Addr().String()
	}
	return prefix.String()
}
