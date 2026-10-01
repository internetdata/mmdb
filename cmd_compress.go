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

Only for legacy mmdb files: ones written without v2 of the Go mmdbwriter, which
%[1]s is built on. compress stores each identical part of the search tree once,
so the file shrinks and every lookup answers as before. A file %[1]s import
writes never needs it, and compress refuses a file that is compressed already.

Example:
  $ %[1]s compress legacy.mmdb compressed.mmdb

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
