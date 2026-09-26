// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
)

// version is the release, set here and checked against the tag at release time.
var version = "1.0.0"

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
	case cmd == "completion":
		err = cmdCompletion()
	case cmd == "version":
		err = cmdVersion()
	default:
		err = cmdDefault()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "err: %v\n", err)
	}
}
