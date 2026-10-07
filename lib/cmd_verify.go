// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

import (
	"errors"
	"fmt"

	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/spf13/pflag"
)

// ErrPrinted is returned by a command that has already printed why it failed,
// as verify does for an invalid file: exit non-zero and print nothing more.
var ErrPrinted = errors.New("failure already printed")

// CmdVerifyFlags are flags expected by CmdVerify.
type CmdVerifyFlags struct {
	Help bool
}

// Init initializes the common flags available to CmdVerify with sensible
// defaults.
//
// pflag.Parse() must be called to actually use the final flag values.
func (f *CmdVerifyFlags) Init() {
	pflag.BoolVarP(
		&f.Help,
		"help", "h", false,
		"show help.",
	)

}

func CmdVerify(f CmdVerifyFlags, args []string, printHelp func()) error {
	// help?
	if f.Help || (pflag.NArg() == 1 && pflag.NFlag() == 0) {
		printHelp()
		return nil
	}

	// validate input file.
	if len(args) == 0 {
		return errors.New("input mmdb file required as first argument")
	}

	// open tree.
	db, err := maxminddb.Open(args[0])
	if err != nil {
		return fmt.Errorf("couldn't open mmdb file: %w", err)
	}
	defer db.Close()

	// verify.
	err = db.Verify()
	if err != nil {
		fmt.Printf("invalid: %v\n", err)
		return ErrPrinted
	}
	fmt.Println("valid")

	return nil
}
