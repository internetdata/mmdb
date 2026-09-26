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

var completionsVerify = &complete.Command{
	Flags: map[string]complete.Predictor{
		"-h":     predict.Nothing,
		"--help": predict.Nothing,
	},
}

func printHelpVerify() {
	fmt.Printf(
		`Usage: %s verify [<opts>] <mmdb_file>

Options:
  General:
    --help, -h
      show help.
`, progBase)
}

func cmdVerify() error {
	f := mmdbLib.CmdVerifyFlags{}
	f.Init()
	pflag.Parse()

	return mmdbLib.CmdVerify(f, pflag.Args()[1:], printHelpVerify)
}
