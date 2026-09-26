// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package main

import (
	"fmt"

	mmdbLib "github.com/internetdata/mmdb/lib"

	complete "github.com/mslmio/libgo-complete"
	"github.com/mslmio/libgo-complete/predict"

	"github.com/spf13/pflag"
)

var predictFormats = []string{"csv", "tsv", "json"}

var completionsExport = &complete.Command{
	Flags: map[string]complete.Predictor{
		"-h":          predict.Nothing,
		"--help":      predict.Nothing,
		"-o":          predict.Nothing,
		"--out":       predict.Nothing,
		"-f":          predict.Set(predictFormats),
		"--format":    predict.Set(predictFormats),
		"--cidr-only": predict.Nothing,
		"--no-header": predict.Nothing,
	},
}

func printHelpExport() {
	fmt.Printf(
		`Usage: %s export [<opts>] <mmdb_file> [<out_file>]

Options:
  General:
    --help, -h
      show help.

  Input/Output:
    -o <fname>, --out <fname>
      output file name. (e.g. out.csv)
      default: <out_file> if specified, otherwise stdout.

  Format:
    -f <format>, --format <format>
      the output file format.
      can be "csv", "tsv" or "json".
      default: csv if output file ends in ".csv", tsv if ".tsv",
      json if ".json", otherwise csv.
    --no-header
      don't output the header for file formats that include one, like
      CSV/TSV/JSON.
      default: false.
    --cidr-only
      require that the range column is in CIDR form, even for single IP rows.
      default: false.
`, progBase)
}

func cmdExport() error {
	f := mmdbLib.CmdExportFlags{}
	f.Init()
	pflag.Parse()

	return mmdbLib.CmdExport(f, pflag.Args()[1:], printHelpExport)
}
