//go:build !unix

package lib

import (
	"io/fs"
	"os"
)

func links(fs.FileInfo) uint64 {
	return 1
}

func takeOwner(*os.File, fs.FileInfo) error {
	return nil
}
