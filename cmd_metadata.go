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

var predictMetadataFmts = []string{"pretty", "json"}

var completionsMetadata = &complete.Command{
	Flags: map[string]complete.Predictor{
		"--nocolor":    predict.Nothing,
		"--data-types": predict.Nothing,
		"-h":           predict.Nothing,
		"--help":       predict.Nothing,
		"-f":           predict.Set(predictMetadataFmts),
		"--format":     predict.Set(predictMetadataFmts),
	},
}

func printHelpMetadata() {
	fmt.Printf(
		`Usage: %s metadata [<opts>] <mmdb_file>

Options:
  General:
    --nocolor
      disable colored output.
    --data-types
      show data type sizes within the data section.
    --help, -h
      show help.

  Format:
    -f <format>, --format <format>
      the metadata output format.
      can be "pretty" or "json".
      default: pretty.
`, progBase)
}

func cmdMetadata() error {
	f := mmdbLib.CmdMetadataFlags{}
	f.Init()
	pflag.Parse()

	return mmdbLib.CmdMetadata(f, pflag.Args()[1:], printHelpMetadata)
}
