package main

import (
	"fmt"

	mmdbLib "github.com/internetdata/mmdb/lib"

	complete "github.com/mslmio/libgo-complete"
	"github.com/mslmio/libgo-complete/predict"

	"github.com/spf13/pflag"
)

var completionsCompress = &complete.Command{
	Flags: map[string]complete.Predictor{
		"-h":     predict.Nothing,
		"--help": predict.Nothing,
	},
}

func printHelpCompress() {
	fmt.Printf(
		`Usage: %s compress [<opts>] <mmdb_file> <out_mmdb_file>

Rewrites an mmdb file so each identical part of its search tree is stored once.
Every lookup answers as before; only the file gets smaller.

A file that already shares those parts, like every one %[1]s import writes,
can't shrink: compress reads just its tree, says it is already compressed and
writes nothing. Otherwise it loads the whole file, taking about as much memory
as importing it.

Example:
  $ %[1]s compress data.mmdb data.small.mmdb

Options:
  General:
    --help, -h
      show help.
`, progBase)
}

func cmdCompress() error {
	f := mmdbLib.CmdCompressFlags{}
	f.Init()
	pflag.Parse()

	return mmdbLib.CmdCompress(f, pflag.Args()[1:], printHelpCompress)
}
