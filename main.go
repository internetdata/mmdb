// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	mmdbLib "github.com/internetdata/mmdb/lib"

	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

// version is the release, set here and checked against the tag at release time.
var version = "1.10.4"

var progBase = filepath.Base(os.Args[0])

// global flags
var fHelp bool
var fNoColor bool

func main() {
	var err error
	var cmd string

	// obey NO_COLOR env var.
	if os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}

	handleCompletions()

	// parseFlags prints a flag error, never pflag's list of every flag.
	pflag.CommandLine.Init(progBase, pflag.ContinueOnError)

	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch {
	case cmd == "read":
		err = cmdRead()
	case cmd == "import":
		err = cmdImport()
	case cmd == "export":
		err = cmdExport()
	case cmd == "diff":
		err = cmdDiff()
	case cmd == "verify":
		err = cmdVerify()
	case cmd == "metadata":
		err = cmdMetadata()
	case cmd == "compress":
		err = cmdCompress()
	case cmd == "completion":
		err = cmdCompletion()
	case cmd == "version":
		err = cmdVersion()
	default:
		err = cmdDefault()
	}

	if err != nil {
		if !errors.Is(err, mmdbLib.ErrPrinted) {
			fmt.Fprintf(os.Stderr, "err: %v\n", err)
		}
		os.Exit(1)
	}
}

// parseFlags parses the command line, and on a bad flag prints the error and
// where cmd's help is, then exits 2; cmd is empty for the general help.
func parseFlags(cmd string) {
	err := pflag.CommandLine.Parse(os.Args[1:])
	if err == nil {
		return
	}
	help := progBase + " --help"
	if cmd != "" {
		help = progBase + " " + cmd + " --help"
	}
	fmt.Fprintf(os.Stderr, "err: %v\nsee '%s'\n", err, help)
	os.Exit(2)
}
