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

var completionsDiff = &complete.Command{
	Flags: map[string]complete.Predictor{
		"-h":        predict.Nothing,
		"--help":    predict.Nothing,
		"-s":        predict.Nothing,
		"--subnets": predict.Nothing,
		"-r":        predict.Nothing,
		"--records": predict.Nothing,
	},
}

func printHelpDiff() {
	fmt.Printf(
		`Usage: %s diff [<opts>] <old> <new>

Description:
  Print subnet and record differences between two mmdb files (i.e. do set
  difference `+"`"+"(new - old) U (old - new)"+"`"+`).

Options:
  General:
    --help, -h
      show help.
    --subnets, -s
      show subnets difference.
    --records, -r
      show records difference.
`, progBase)
}

func cmdDiff() error {
	f := mmdbLib.CmdDiffFlags{}
	f.Init()
	pflag.Parse()

	return mmdbLib.CmdDiff(f, pflag.Args()[1:], printHelpDiff)
}
